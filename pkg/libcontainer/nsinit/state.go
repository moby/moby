package nsinit

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// StateWriter handles writing and deleting the pid file
// on disk
type StateWriter interface {
	WritePid(pid int, startTime string) error
	DeletePid() error
}

type DefaultStateWriter struct {
	Root string
}

// writePidFile writes the namespaced processes pid to pid in the rootfs for the container
func (d *DefaultStateWriter) WritePid(pid int, startTime string) error {
	err := ioutil.WriteFile(filepath.Join(d.Root, "pid"), []byte(fmt.Sprint(pid)), 0655)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(d.Root, "start"), []byte(startTime), 0655)
}

func (d *DefaultStateWriter) DeletePid() error {
	err := os.Remove(filepath.Join(d.Root, "pid"))
	if serr := os.Remove(filepath.Join(d.Root, "start")); err == nil {
		err = serr
	}
	return err
}
