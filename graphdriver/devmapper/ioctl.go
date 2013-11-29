package devmapper

import (
	"unsafe"
)

func ioctlLoopCtlGetFree(fd uintptr) (int, error) {
	index, _, err := sysSyscall(sysSysIoctl, fd, LoopCtlGetFree, 0)
	if err != 0 {
		return 0, err
	}
	return int(index), nil
}

func ioctlLoopSetFd(loopFd, sparseFd uintptr) error {
	if _, _, err := sysSyscall(sysSysIoctl, loopFd, LoopSetFd, sparseFd); err != 0 {
		return err
	}
	return nil
}

func ioctlLoopSetStatus64(loopFd uintptr, loopInfo *LoopInfo64) error {
	if _, _, err := sysSyscall(sysSysIoctl, loopFd, LoopSetStatus64, uintptr(unsafe.Pointer(loopInfo))); err != 0 {
		return err
	}
	return nil
}

func ioctlLoopClrFd(loopFd uintptr) error {
	if _, _, err := sysSyscall(sysSysIoctl, loopFd, LoopClrFd, 0); err != 0 {
		return err
	}
	return nil
}

func ioctlLoopGetStatus64(loopFd uintptr) (*LoopInfo64, error) {
	loopInfo := &LoopInfo64{}

	if _, _, err := sysSyscall(sysSysIoctl, loopFd, LoopGetStatus64, uintptr(unsafe.Pointer(loopInfo))); err != 0 {
		return nil, err
	}
	return loopInfo, nil
}

func ioctlLoopSetCapacity(loopFd uintptr, value int) error {
	if _, _, err := sysSyscall(sysSysIoctl, loopFd, LoopSetCapacity, uintptr(value)); err != 0 {
		return err
	}
	return nil
}

func ioctlBlkGetSize64(fd uintptr) (int64, error) {
	var size int64
	if _, _, err := sysSyscall(sysSysIoctl, fd, BlkGetSize64, uintptr(unsafe.Pointer(&size))); err != 0 {
		return 0, err
	}
	return size, nil
}
