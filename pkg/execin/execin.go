package execin

import (
	"os"
	"runtime"

	"github.com/docker/libcontainer/system"
)

func ExecIn(execFile *os.File, cloneOpts uintptr, enterFunc func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := system.Setns(execFile.Fd(), cloneOpts); err != nil {
		return err
	}

	return enterFunc()
}
