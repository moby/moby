package archive

import (
	"syscall"
	"unsafe"
)

func getLastAccess(stat *syscall.Stat_t) syscall.Timespec {
	return stat.Atim
}

func getLastModification(stat *syscall.Stat_t) syscall.Timespec {
	return stat.Mtim
}

func LUtimesNano(path string, ts []syscall.Timespec) error {
	// These are not currently availible in syscall
	AT_FDCWD := -100
	AT_SYMLINK_NOFOLLOW := 0x100

	var _path *byte
	_path, err := syscall.BytePtrFromString(path)
	if err != nil {
		return err
	}

	if _, _, err := syscall.Syscall6(syscall.SYS_UTIMENSAT, uintptr(AT_FDCWD), uintptr(unsafe.Pointer(_path)), uintptr(unsafe.Pointer(&ts[0])), uintptr(AT_SYMLINK_NOFOLLOW), 0, 0); err != 0 && err != syscall.ENOSYS {
		return err
	}

	return nil
}
