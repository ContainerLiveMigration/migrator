package apptainer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"strings"
	"syscall"

	"path/filepath"
)

const (
	instancePath = "instances"
	apptainerDir = ".apptainer"
	ProgPrefix   = "Apptainer instance"
	AppSubDir    = "app"
)

// File represents an instance file storing instance information
type File struct {
	Path       string `json:"-"`
	Pid        int    `json:"pid"`
	PPid       int    `json:"ppid"`
	Name       string `json:"name"`
	User       string `json:"user"`
	Image      string `json:"image"`
	Config     []byte `json:"config"`
	UserNs     bool   `json:"userns"`
	Cgroup     bool   `json:"cgroup"`
	IP         string `json:"ip"`
	LogErrPath string `json:"logErrPath"`
	LogOutPath string `json:"logOutPath"`
	Checkpoint string `json:"checkpoint"`
}

// Delete deletes instance file
func (i *File) Delete() error {
	dir := filepath.Dir(i.Path)
	if dir == "." {
		dir = ""
	}
	return os.RemoveAll(dir)
}

// isExited returns if the instance process is exited or not.
func (i *File) isExited() bool {
	if i.PPid <= 0 {
		return true
	}

	// if instance is not running anymore, automatically
	// delete instance files after checking that instance
	// parent process
	err := syscall.Kill(i.PPid, 0)
	if err == syscall.ESRCH {
		return true
	} else if err == nil {
		// process is alive and is owned by you otherwise
		// we would have obtained permission denied error,
		// now check if it's an instance parent process
		cmdline := fmt.Sprintf("/proc/%d/cmdline", i.PPid)
		d, err := ioutil.ReadFile(cmdline)
		if err != nil {
			// this is racy and not accurate but as the process
			// may have exited during above read, check again
			// for process presence
			return syscall.Kill(i.PPid, 0) == syscall.ESRCH
		}
		// not an instance master process
		return !strings.HasPrefix(string(d), ProgPrefix)
	}

	return false
}

// Update stores instance information in associated instance file
func (i *File) Update() error {
	b, err := json.Marshal(i)
	if err != nil {
		return err
	}

	path := filepath.Dir(i.Path)

	oldumask := syscall.Umask(0)
	defer syscall.Umask(oldumask)

	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(i.Path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|syscall.O_NOFOLLOW, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Write(b); err != nil {
		return fmt.Errorf("failed to write instance file %s: %s", i.Path, err)
	}

	return file.Sync()
}

// getPath returns the path where searching for instance files
func getPath(username string, subDir string) (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}

	u, err := user.Lookup(username)
	if err != nil {
		log.Printf("failed to lookup user %s: %s", username, err)
		return "", err
	}

	return filepath.Join(u.HomeDir, apptainerDir, instancePath, subDir, hostname, username), nil
}

// List returns instance files matching username and/or name pattern
func List(username string, name string, subDir string) ([]*File, error) {
	list := make([]*File, 0)

	path, err := getPath(username, subDir)
	if err != nil {
		log.Printf("failed to get instance path: %s", err)
		return nil, err
	}
	log.Printf("instances path: %s", path)
	pattern := filepath.Join(path, name, name+".json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		r, err := os.Open(file)
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}
		f := &File{}
		if err := json.NewDecoder(r).Decode(f); err != nil {
			r.Close()
			return nil, err
		}
		r.Close()
		f.Path = file
		// delete ghost apptainer instance files
		if subDir == AppSubDir && f.isExited() {
			f.Delete()
			continue
		}
		list = append(list, f)
	}
	return list, nil
}

// Get returns the instance file corresponding to instance name
func Get(userName string, instanceName string, subDir string) (*File, error) {
	list, err := List(userName, instanceName, subDir)
	if err != nil {
		return nil, err
	}
	if len(list) != 1 {
		return nil, fmt.Errorf("no instance found with name %s", instanceName)
	}
	return list[0], nil
}
