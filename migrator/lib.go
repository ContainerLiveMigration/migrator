package migrator

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"

	"cr/apptainer"
)

// mountTmpfs mounts a tmpfs at the given path
func mountTmpfs(path string) error {
	return syscall.Mount("none", path, "tmpfs", 0, "")
}

// unmountTmpfs unmounts the tmpfs at the given path
func unmountTmpfs(path string) error {
	return syscall.Unmount(path, 0)
}

// get user id of the given user name
func getUIDAndGID(name string) (int64, int64, error) {
	u, err := user.Lookup(name)
	if err != nil {
		return -1, -1, fmt.Errorf("user %s not found", name)
	}
	uid, err := strconv.ParseInt(u.Uid, 10, 32)
	if err != nil {
		return -1, -1, fmt.Errorf("failed to parse user id: %v", err)
	}
	gid, err := strconv.ParseInt(u.Gid, 10, 32)
	if err != nil {
		return -1, -1, fmt.Errorf("failed to parse group id: %v", err)
	}
	return uid, gid, nil
}

// execute command as another user
func executeAsUser(cmd *exec.Cmd, user string) error {
	uid, gid, err := getUIDAndGID(user)
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Printf("username %s, uid %d, gid %d", user, uid, gid)
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
	return cmd.Run()
}

func apptainerRestore(instanceName string, userName string) error {
	cmd := exec.Command("apptainer", "checkpoint", "instance", "--criu", "--restore", instanceName)
	return executeAsUser(cmd, userName)
}

func getContainerStatus(userName string, instanceName string) (*apptainer.File, error) {
	file, err := apptainer.Get(userName, instanceName, apptainer.AppSubDir)
	if err != nil {
		log.Printf("failed to get instance %s: %v", instanceName, err)
		return nil, err
	}
	return file, nil
}
