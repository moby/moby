//go:build linux
// +build linux

package overlay2 // import "github.com/docker/docker/daemon/graphdriver/overlay2"

import (
	"runtime"

	"golang.org/x/sys/unix"
)

func mountFrom(dir, device, target, mType string, flags uintptr, label string) error {
	chErr := make(chan error, 1)

	go func() {
		runtime.LockOSThread()
		// Do not unlock this thread as the thread state cannot be restored
		// We do not want go to re-use this thread for anything else.

		if err := unix.Unshare(unix.CLONE_FS); err != nil {
			chErr <- err
			return
		}
		if err := unix.Chdir(dir); err != nil {
			chErr <- err
			return
		}
		chErr <- unix.Mount(device, target, mType, flags, label)
	}()
	return <-chErr
}
