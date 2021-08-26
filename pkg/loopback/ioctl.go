//go:build linux
// +build linux

package loopback // import "github.com/docker/docker/pkg/loopback"

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

func ioctlLoopCtlGetFree(fd uintptr) (int, error) {
	// The ioctl interface for /dev/loop-control (since Linux 3.1) is a bit
	// off compared to what you'd expect: instead of writing an integer to a
	// parameter pointer like unix.IoctlGetInt() expects, it returns the first
	// available loop device index directly.
	ioctlReturn, _, err := unix.Syscall(unix.SYS_IOCTL, fd, LoopCtlGetFree, 0)
	if err != 0 {
		return 0, err
	}
	return int(ioctlReturn), nil
}

func ioctlLoopSetFd(loopFd, sparseFd uintptr) error {
	return unix.IoctlSetInt(int(loopFd), unix.LOOP_SET_FD, int(sparseFd))
}

func ioctlLoopSetStatus64(loopFd uintptr, loopInfo *unix.LoopInfo64) error {
	if _, _, err := unix.Syscall(unix.SYS_IOCTL, loopFd, unix.LOOP_SET_STATUS64, uintptr(unsafe.Pointer(loopInfo))); err != 0 {
		return err
	}
	return nil
}

func ioctlLoopClrFd(loopFd uintptr) error {
	if _, _, err := unix.Syscall(unix.SYS_IOCTL, loopFd, unix.LOOP_CLR_FD, 0); err != 0 {
		return err
	}
	return nil
}

func ioctlLoopGetStatus64(loopFd uintptr) (*unix.LoopInfo64, error) {
	loopInfo := &unix.LoopInfo64{}

	if _, _, err := unix.Syscall(unix.SYS_IOCTL, loopFd, unix.LOOP_GET_STATUS64, uintptr(unsafe.Pointer(loopInfo))); err != 0 {
		return nil, err
	}
	return loopInfo, nil
}

func ioctlLoopSetCapacity(loopFd uintptr, value int) error {
	return unix.IoctlSetInt(int(loopFd), unix.LOOP_SET_CAPACITY, value)
}
