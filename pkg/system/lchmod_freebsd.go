package system

import (
	"os"
	"syscall"
	"unsafe"
)

func LChmod(path string, mode os.FileMode) error {
	var _path *byte
	_path, err := syscall.BytePtrFromString(path)
	if err != nil {
		return err
	}

	if _, _, err := syscall.Syscall(syscall.SYS_LCHMOD, uintptr(unsafe.Pointer(_path)), uintptr(mode), 0); err != 0 {
		return err
	}

	return nil
}
