package sysfs

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"

	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
)

const _UTIME_OMIT = unix.UTIME_OMIT

func utimens(path string, atim, mtim int64) experimentalsys.Errno {
	times := timesToTimespecs(atim, mtim)
	if times == nil {
		return 0
	}
	return experimentalsys.UnwrapOSError(syscall.UtimesNano(path, times[:]))
}

// On linux, implement futimens via utimensat with the NUL path.
func futimens(fd uintptr, atim, mtim int64) experimentalsys.Errno {
	times := timesToTimespecs(atim, mtim)
	if times == nil {
		return 0
	}
	return experimentalsys.UnwrapOSError(utimensat(int(fd), 0 /* NUL */, times, 0))
}

// utimensat is like syscall.utimensat special-cased to accept a NUL string for the path value.
func utimensat(dirfd int, strPtr uintptr, times *[2]syscall.Timespec, flags int) (err error) {
	_, _, e1 := syscall.Syscall6(syscall.SYS_UTIMENSAT, uintptr(dirfd), strPtr, uintptr(unsafe.Pointer(times)), uintptr(flags), 0, 0)
	if e1 != 0 {
		err = e1
	}
	return
}
