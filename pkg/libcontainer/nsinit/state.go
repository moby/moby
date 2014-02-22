package nsinit

import (
	"fmt"
	"io/ioutil"
	"os"
)

type StateWriter interface {
	WritePid(pid int) error
	DeletePid() error
}

type DefaultStateWriter struct {
}

// writePidFile writes the namespaced processes pid to .nspid in the rootfs for the container
func (*DefaultStateWriter) WritePid(pid int) error {
	return ioutil.WriteFile(".nspid", []byte(fmt.Sprint(pid)), 0655)
}

func (*DefaultStateWriter) DeletePid() error {
	return os.Remove(".nspid")
}
