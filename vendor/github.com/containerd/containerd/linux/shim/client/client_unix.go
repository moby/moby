// +build !linux,!windows

package client

import (
	"os/exec"
	"syscall"
)

func getSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func setCgroup(cgroupPath string, cmd *exec.Cmd) error {
	return nil
}
