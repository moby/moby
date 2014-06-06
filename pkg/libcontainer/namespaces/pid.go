package namespaces

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// WritePid writes the namespaced processes pid to pid and it's start time
// to the path specified
func WritePid(path string, pid int, startTime string) error {
	err := ioutil.WriteFile(filepath.Join(path, "pid"), []byte(fmt.Sprint(pid)), 0655)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(path, "start"), []byte(startTime), 0655)
}

// DeletePid removes the pid and started file from disk when the container's process
// dies and the container is cleanly removed
func DeletePid(path string) error {
	err := os.Remove(filepath.Join(path, "pid"))
	if serr := os.Remove(filepath.Join(path, "start")); err == nil {
		err = serr
	}
	return err
}
