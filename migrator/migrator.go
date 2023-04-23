package migrator

import (
	"cr/apptainer"
	"cr/util"
	"encoding/binary"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/rpc"
	"os/exec"
	"path/filepath"
)

type Migrator struct {
	IsSharedFS bool
}

func (m *Migrator) Migrate(req *MigrateRequest, res *MigrateResponse) error {
	log.Printf("migrate request received: %v", req)
	// 1. dump the container
	cmd := exec.Command(
		"apptainer",
		"checkpoint",
		"instance",
		"--criu",
		req.InstanceName,
	)

	err := cmd.Run()
	if err != nil {
		log.Printf("failed to dump instance %s: %v", req.InstanceName, err)
	}

	instance, err := apptainer.GetContainerStatus(req.UserName, req.InstanceName)
	if err != nil {
		log.Printf("failed to get checkpoint name of instance %s: %v", req.InstanceName, err)
		res.Status = FAIL
		return err
	}

	log.Printf("dump instance %s to checkpoint %s", req.InstanceName, instance.Checkpoint)

	// 2. stop the container
	cmd = exec.Command(
		"apptainer",
		"instance",
		"stop",
		req.InstanceName,
	)
	// don't wait for the command to finish
	go func() {
		err = cmd.Run()
		if err != nil {
			log.Printf("failed to stop instance %s: %v", req.InstanceName, err)
		}
	}()

	// 3. if not in shared filesystem, rsync the checkpoint to the target
	if m.IsSharedFS {
		// 3.1 get the checkpoint dir
		checkpointDir, err := apptainer.GetCheckpointDir(req.UserName, instance.Checkpoint)
		if err != nil {
			log.Printf("failed to get checkpoint dir of instance %s: %v", req.InstanceName, err)
			res.Status = FAIL
			return err
		}

		// 3.2 run rsync
		err = util.DoRsync(req.UserName, checkpointDir, req.Target)
		if err != nil {
			log.Printf("failed to rsync checkpoint %s to %s: %v", instance.Checkpoint, req.Target, err)
			res.Status = FAIL
			return err
		}
	}

	// 3. request the server to restore the container
	client, err := rpc.DialHTTP("tcp", req.Target+RPCPort)
	if err != nil {
		log.Printf("failed to connect to server: %v", err)
		res.Status = FAIL
		return err
	}
	defer client.Close()

	r := RestartContainerResponse{}

	err = client.Call("Migrator.RestartContainer", &RestartContainerRequest{
		UserName:       req.UserName,
		InstanceName:   req.InstanceName,
		CheckpointName: instance.Checkpoint,
		ImagePath:      instance.Image,
	}, &r)

	if err != nil || r.Status != OK {
		log.Printf("failed to restart container %s: %v", req.InstanceName, err)
		res.Status = FAIL
		return err
	}
	log.Printf("restart container %s successfully", req.InstanceName)
	res.Status = OK
	return nil
}

func (m *Migrator) DisklessMigrate(req *DisklessMigrateRequest, res *DisklessMigrateResponse) error {
	log.Printf("diskless migrate request: %v", req)
	// 1. check if the checkpoint is memory mode
	instance, err := apptainer.GetContainerStatus(req.UserName, req.InstanceName)
	if err != nil {
		log.Printf("failed to get checkpoint name of instance %s: %v", req.InstanceName, err)
		res.Status = FAIL
		return err
	}
	checkpointDir, err := apptainer.GetCheckpointDir(req.UserName, instance.Checkpoint)
	if err != nil {
		log.Printf("failed to get checkpoint dir of instance %s: %v", req.InstanceName, err)
		res.Status = FAIL
		return err
	}
	imgDir, err := apptainer.GetImageRealPath(checkpointDir)
	if err != nil {
		log.Printf("failed to get real path of checkpoint %s: %v", instance.Checkpoint, err)
		res.Status = FAIL
		return err
	}

	log.Printf("image is stored at %v", imgDir)

	// 2. if not in shared filesystem, rsync the checkpointDir to the target
	if m.IsSharedFS {
		err = util.DoRsync(req.UserName, checkpointDir, req.Target)
		if err != nil {
			log.Printf("failed to rsync checkpoint %s to %s: %v", instance.Checkpoint, req.Target, err)
			res.Status = FAIL
			return err
		}
	}

	// 3. request the dest node to launch a page server
	client, err := rpc.DialHTTP("tcp", req.Target+RPCPort)
	if err != nil {
		log.Printf("failed to connect to server %v:%v: %v", req.Target, RPCPort, err)
		res.Status = FAIL
		return err
	}
	pageServerRes := LaunchPageServerResponse{}
	err = client.Call("Migrator.LaunchPageServer", &LaunchPageServerRequest{
		UserName:       req.UserName,
		InstanceName:   req.InstanceName,
		CheckpointName: instance.Checkpoint,
		ImagePath:      instance.Image,
	}, &pageServerRes)
	if err != nil || pageServerRes.Status != OK {
		log.Printf("failed to launch page server: %v", err)
		res.Status = FAIL
		return err
	}
	log.Printf("page server launched successfully")

	// 4. dump the container, criu will send pages to the page server,
	// and store other files in the tmpfs
	cmd := exec.Command(
		"apptainer",
		"checkpoint",
		"instance",
		"--criu",
		"--page-server",
		"--address",
		req.Target,
		req.InstanceName,
	)
	err = cmd.Run()
	if err != nil {
		log.Printf("failed to dump instance %s: %v", req.InstanceName, err)
	}
	log.Printf("dump container successfully")

	// 5. if not in sharedFS, rsync some log files to the server
	if m.IsSharedFS {
		err = util.DoRsync(req.UserName, checkpointDir, req.Target)
		if err != nil {
			log.Printf("failed to rsync checkpoint %s to %s: %v", instance.Checkpoint, req.Target, err)
			res.Status = FAIL
			return err
		}
	}

	// 6. stop the container
	cmd = exec.Command(
		"apptainer",
		"instance",
		"stop",
		req.InstanceName,
	)

	go func() {
		err = cmd.Run()
		if err != nil {
			log.Printf("failed to stop instance %s: %v", req.InstanceName, err)
		}
	}()

	// 7. send other files to the server
	err = sendImages(req.Target+FilePort, imgDir, req.UserName)
	if err != nil {
		log.Printf("failed to send images to server: %v", err)
		res.Status = FAIL
		return err
	}
	log.Printf("send images successfully")

	// 8. request the server to restore
	restoreRes := RestoreResponse{}
	err = client.Call("Migrator.Restore", &RestoreRequest{
		UserName:     req.UserName,
		InstanceName: req.InstanceName,
	}, &restoreRes)
	if err != nil || restoreRes.Status != OK {
		log.Printf("failed to restore container %s: %v", req.InstanceName, err)
		res.Status = FAIL
		return err
	}
	log.Printf("restore container successfully")
	res.Status = OK
	return nil
}

func (m *Migrator) RestartContainer(req *RestartContainerRequest, res *RestartContainerResponse) error {
	cmd := exec.Command(
		"apptainer",
		"instance",
		"start",
		"--criu-restart",
		req.CheckpointName,
		req.ImagePath,
		req.InstanceName,
	)
	err := cmd.Run()
	if err != nil {
		log.Printf("failed to restart instance %s: %v", req.InstanceName, err)
		res.Status = FAIL
		return err
	}
	res.Status = OK
	return nil
}

func (m *Migrator) LaunchPageServer(req *LaunchPageServerRequest, res *LaunchPageServerResponse) error {
	// 1. config checkpoint as memory mode
	cmd := exec.Command(
		"apptainer",
		"checkpoint",
		"config",
		req.CheckpointName,
		"memory",
	)
	err := cmd.Run()
	if err != nil {
		log.Printf("failed to config checkpoint as memory mode: %v", err)
		res.Status = FAIL
		return err
	}
	log.Printf("checkpoint configured as memory mode successfully")

	// 2. launch page server
	cmd = exec.Command(
		"apptainer",
		"instance",
		"start",
		"--criu-restart",
		req.CheckpointName,
		"--page-server",
		req.ImagePath,
		req.InstanceName,
	)
	err = cmd.Run()
	if err != nil {
		log.Printf("failed to launch page server: %v", err)
		res.Status = FAIL
		return err
	}
	log.Printf("page server launched successfully")
	res.Status = OK
	return nil
}

func (m *Migrator) AsyncImgs() error {
	return nil
}

func (m *Migrator) Restore(req *RestoreRequest, res *RestoreResponse) error {
	cmd := exec.Command(
		"apptainer",
		"checkpoint",
		"instance",
		"--criu",
		"--restore",
		req.InstanceName,
	)
	err := cmd.Run()
	if err != nil {
		res.Status = FAIL
		log.Printf("failed to restore instance %s: %v", req.InstanceName, err)
		return err
	}
	log.Printf("restore container successfully")
	res.Status = OK
	return nil
}

// TODO: maybe we can simplify transfering images by using rsync
func sendImages(addr string, imgDir string, userName string) error {
	// 1. connect to the server
	client, err := net.Dial("tcp", addr)
	if err != nil {
		log.Printf("failed to connect to server %v: %v", addr, err)
		return err
	}
	defer client.Close()
	// 2. tar the images
	tarballPath := filepath.Join(imgDir, "img.tar.gz")
	var imageFiles []string
	// get all file names under imgDir
	files, err := ioutil.ReadDir(imgDir)
	if err != nil {
		log.Printf("read all files failes, %v", err)
		return err
	}
	for _, f := range files {
		imageFiles = append(imageFiles, f.Name())
	}
	tarCmd := exec.Command("tar", append([]string{"-zcf", "img.tar.gz"}, imageFiles...)...)
	tarCmd.Dir = imgDir
	err = tarCmd.Run()
	if err != nil {
		return err
	}
	log.Printf("tar images at %v successfully", imgDir)

	// 3. send tarball to server
	// 3.1. send 4 bytes as integer as the length of the tarball path
	fileNameLength := int32(len(tarballPath))
	err = binary.Write(client, binary.LittleEndian, &fileNameLength)
	if err != nil {
		log.Printf("failed to read file name length: %v", err)
		return err
	}
	// 3.2. send the tarball path
	// TODO: encode the file name
	io.WriteString(client, tarballPath)
	if err != nil {
		log.Printf("failed to read file name: %v", err)
		return err
	}

	// 3.3. send tarball
	err = util.SendFile(client, tarballPath)
	log.Printf("send tarball %s successfully", tarballPath)
	if err != nil {
		log.Printf("failed to receive file: %v", err)
		return err
	}
	return nil
}
