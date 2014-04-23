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

func GetClockTicks() int {
	// when we cannot call out to C to get the sysconf it is fairly safe to
	// just return 100
	return 100
}
