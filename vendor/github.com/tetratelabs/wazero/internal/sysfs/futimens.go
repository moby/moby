//go:build (linux || darwin) && !tinygo

package sysfs

import (
	"syscall"
	"unsafe"

	"github.com/tetratelabs/wazero/experimental/sys"
)

func timesToPtr(times *[2]syscall.Timespec) unsafe.Pointer { //nolint:unused
	if times != nil {
		return unsafe.Pointer(&times[0])
	}
	return unsafe.Pointer(nil)
}

func timesToTimespecs(atim int64, mtim int64) (times *[2]syscall.Timespec) {
	// When both inputs are omitted, there is nothing to change.
	if atim == sys.UTIME_OMIT && mtim == sys.UTIME_OMIT {
		return
	}

	times = &[2]syscall.Timespec{}
	if atim == sys.UTIME_OMIT {
		times[0] = syscall.Timespec{Nsec: _UTIME_OMIT}
		times[1] = syscall.NsecToTimespec(mtim)
	} else if mtim == sys.UTIME_OMIT {
		times[0] = syscall.NsecToTimespec(atim)
		times[1] = syscall.Timespec{Nsec: _UTIME_OMIT}
	} else {
		times[0] = syscall.NsecToTimespec(atim)
		times[1] = syscall.NsecToTimespec(mtim)
	}
	return
}
