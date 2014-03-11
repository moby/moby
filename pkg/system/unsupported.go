// +build !linux

package system

import (
	"os/exec"
)

func SetCloneFlags(cmd *exec.Cmd, flag uintptr) {

}

func ParentDeathSignal() error {
	return ErrNotSupportedPlatform
}

func UsetCloseOnExec(fd uintptr) error {
	return ErrNotSupportedPlatform
}
