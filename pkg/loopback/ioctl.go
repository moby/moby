// +build linux

package loopback // import "github.com/docker/docker/pkg/loopback"

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

func ioctlLoopCtlGetFree(fd uintptr) (int, error) {
	index, err := unix.IoctlGetInt(int(fd), LoopCtlGetFree)
	if err != nil {
		return 0, err
	}
	return index, nil
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
