package mount

import (
	"syscall"
)

func mount(device, target, mType string, flag uintptr, data string) error {
	return syscall.Mount(device, target, mType, flag, data)
}

func unmount(target string, flag int) error {
	return syscall.Unmount(target, flag)
}
