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
	WritePid(pid int) error
	DeletePid() error
}

type DefaultStateWriter struct {
	Root string
}

// writePidFile writes the namespaced processes pid to .nspid in the rootfs for the container
func (d *DefaultStateWriter) WritePid(pid int) error {
	return ioutil.WriteFile(filepath.Join(d.Root, ".nspid"), []byte(fmt.Sprint(pid)), 0655)
}

func (d *DefaultStateWriter) DeletePid() error {
	return os.Remove(filepath.Join(d.Root, ".nspid"))
}
