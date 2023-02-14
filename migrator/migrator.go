package migrator

import (
	"cr/apptainer"
	"log"
	"net/rpc"
	"os/exec"
)

type Migrator struct {
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

	err := executeAsUser(cmd, req.UserName)
	if err != nil {
		log.Printf("failed to dump instance %s: %v", req.InstanceName, err)
	}

	instance, err := getContainerStatus(req.UserName, req.InstanceName)
	if err != nil {
		log.Printf("failed to get checkpoint name of instance %s: %v", req.InstanceName, err)
		res.Status = FAIL
		return err
	}

	// 2. stop the container
	cmd = exec.Command(
		"apptainer",
		"instance",
		"stop",
		req.InstanceName,
	)

	// 3. request the server to restore the container
	err = executeAsUser(cmd, req.UserName)
	if err != nil {
		log.Printf("failed to stop instance %s: %v", req.InstanceName, err)
	}

	client, err := rpc.DialHTTP("tcp", req.Target+Port)
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

	res.Status = OK
	return nil
}

func (m *Migrator) DisklessMigrate(req *DisklessMigrateRequest, res *DisklessMigrateResponse) error {
	log.Printf("diskless migrate request: %v", req)
	// 1. mount tmpfs at dir which store the checkpoint
	instance, err := getContainerStatus(req.UserName, req.InstanceName)
	if err != nil {
		log.Printf("get container status failed: %v", err)
		res.Status = FAIL
		return err
	}
	checkpointDir, err := apptainer.GetCheckpointDir(req.UserName, instance.Checkpoint)
	if err != nil {
		log.Printf("get checkpoint dir failed: %v", err)
		res.Status = FAIL
		return err
	}
	imgDir := checkpointDir + "/img"
	err = mountTmpfs(imgDir)
	if err != nil {
		log.Printf("mount tmpfs at %v failed: %v", imgDir, err)
		res.Status = FAIL
		return err
	}
	log.Printf("mount tmpfs at %v", imgDir)
	// 2. request the server to launch a page server
	client, err := rpc.DialHTTP("tcp", req.Target+Port)
	if err != nil {
		log.Printf("failed to connect to server %v:%v: %v", req.Target, Port, err)
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

	// 3. dump the container, criu will send pages to the page server,
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
	err = executeAsUser(cmd, req.UserName)
	if err != nil {
		log.Printf("failed to dump instance %s: %v", req.InstanceName, err)

	}
	log.Printf("dump container successfully")
	// 4. stop the container
	cmd = exec.Command(
		"apptainer",
		"instance",
		"stop",
		req.InstanceName,
	)
	err = executeAsUser(cmd, req.UserName)
	if err != nil {
		log.Printf("failed to stop instance %s: %v", req.InstanceName, err)
	}
	log.Printf("stop container successfully")

	// 5. send other files to the server, and request the server to restore
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
	err := executeAsUser(cmd, req.UserName)
	if err != nil {
		log.Printf("failed to restart instance %s: %v", req.InstanceName, err)
		res.Status = FAIL
		return err
	}
	res.Status = OK
	return nil
}

func (m *Migrator) LaunchPageServer(req *LaunchPageServerRequest, res *LaunchPageServerResponse) error {

	// 1. mount tmpfs at dir which store the checkpoint
	checkpointDir, err := apptainer.GetCheckpointDir(req.UserName, req.CheckpointName)
	if err != nil {
		log.Printf("failed to get checkpoint dir: %v", err)
		res.Status = FAIL
		return err
	}
	imgDir := checkpointDir + "/img"
	err = mountTmpfs(imgDir)
	log.Printf("mount tmpfs at %v", imgDir)
	if err != nil {
		log.Printf("failed to mount tmpfs at %v: %v", imgDir, err)
		res.Status = FAIL
		return err
	}

	// 2. launch page server
	cmd := exec.Command(
		"apptainer",
		"instance",
		"start",
		"--criu-restart",
		req.CheckpointName,
		"--page-server",
		req.ImagePath,
		req.InstanceName,
	)
	err = executeAsUser(cmd, req.UserName)
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
	err := executeAsUser(cmd, req.UserName)
	if err != nil {
		res.Status = FAIL
		log.Printf("failed to restore instance %s: %v", req.InstanceName, err)
		return err
	}
	log.Printf("restore container successfully")
	res.Status = OK
	return nil
}
