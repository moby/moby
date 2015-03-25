package pidfile

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

type PidFile struct {
	path string
}

func checkPidFileAlreadyExists(path string) error {
	if pidString, err := ioutil.ReadFile(path); err == nil {
		if pid, err := strconv.Atoi(string(pidString)); err == nil {
			if _, err := os.Stat(filepath.Join("/proc", string(pid))); err == nil {
				return fmt.Errorf("pid file found, ensure docker is not running or delete %s", path)
			}
		}
	}
	return nil
}

func New(path string) (file *PidFile, err error) {
	if err := checkPidFileAlreadyExists(path); err != nil {
		return nil, err
	}

	file = &PidFile{path: path}
	err = ioutil.WriteFile(path, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)

	return file, err
}

func (file PidFile) Remove() error {
	if err := os.Remove(file.path); err != nil {
		log.Printf("Error removing %s: %s", file.path, err)
		return err
	}
	return nil
}
