package util

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

// MountTmpfs mounts a tmpfs at the given path
func MountTmpfs(path string) error {
	return syscall.Mount("none", path, "tmpfs", 0, "")
}

// UnmountTmpfs unmounts the tmpfs at the given path
func UnmountTmpfs(path string) error {
	return syscall.Unmount(path, 0)
}

// CreateSymLink creates a symbolic link at the given path
func CreateSymLink(oldname, newname string) error {
	// first remove the new name
	err := os.RemoveAll(newname)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Symlink(oldname, newname)
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
func RunCmdAsUser(cmd *exec.Cmd, user string) error {
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

func SendFile(conn net.Conn, filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		log.Printf("open file %s failed: %v", filepath, err)
		return err
	}
	defer file.Close()
	_, err = io.Copy(conn, file)
	return err
}

func ReceiveFile(conn net.Conn, filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		log.Printf("create file %v failed: %v", filepath, err)
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, conn)
	return err
}

func DoRsync(userName, checkpointDir, targetIP string) error {
	cmd := exec.Command(
		"rsync",
		"-av",
		checkpointDir,
		userName+"@"+targetIP+checkpointDir,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
