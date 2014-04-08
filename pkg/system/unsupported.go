// +build !linux

package system

import (
	"os/exec"
)

func SetCloneFlags(cmd *exec.Cmd, flag uintptr) {

}

func UsetCloseOnExec(fd uintptr) error {
	return ErrNotSupportedPlatform
}

func Gettid() int {
	return 0
}
