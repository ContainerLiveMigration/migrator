package migrator

import (
	"log"
	"net/rpc"
	"os/exec"
)

type Migrator struct {
}

func (m *Migrator) Migrate(req *MigrateRequest, res *MigrateResponse) error {
	log.Printf("migrate request received: %v", req)
	// 1. dump the container
	cmd := exec.Command("apptainer", "checkpoint", "instance", "--criu", req.InstanceName)
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

func (m *Migrator) DisklessMigrate() error {
	// 1. mount tmpfs at dir which store the checkpoint
	// mountTmpfs()

	// 2. request the server to launch a page server

	// 3. dump the container, criu will send pages to the page server,
	// and store other files in the tmpfs

	// 4. send other files to the server, and request the server to restore
	return nil
}

func (m *Migrator) LaunchPageServer(req *LaunchPageServerRequest, res *LaunchPageServerResponse) error {
	// mountTmpfs()
	cmd := exec.Command("apptainer", "instance", "--criu-restart", req.CheckpointName, "--page-server", req.ImagePath, req.InstanceName)
	err := executeAsUser(cmd, req.UserName)
	if err != nil {
		log.Printf("failed to launch page server: %v", err)
		res.Status = FAIL
		return err
	}
	return nil
}

func (m *Migrator) AsyncImgs() error {
	return nil
}

func (m *Migrator) RestartContainer(req *RestartContainerRequest, res *RestartContainerResponse) error {
	cmd := exec.Command("apptainer", "instance", "start", "--criu-restart", req.CheckpointName, req.ImagePath, req.InstanceName)
	err := executeAsUser(cmd, req.UserName)
	if err != nil {
		log.Printf("failed to restart instance %s: %v", req.InstanceName, err)
		res.Status = FAIL
		return err
	}
	res.Status = OK
	return nil
}

func (m *Migrator) Restore(req *RestoreRequest, res *RestoreResponse) error {
	err := apptainerRestore(req.InstanceName, req.UserName)
	if err != nil {
		res.Status = FAIL
		log.Printf("failed to restore instance %s: %v", req.InstanceName, err)
		return err
	}
	res.Status = OK
	return nil
}
