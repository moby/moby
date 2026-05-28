//go:build openbsd && cgo
// +build openbsd,cgo

/*
   Due to how OpenBSD mount(2) works, filesystem types need to be
   supported explicitly since it uses separate structs to pass
   filesystem-specific arguments.

   For now only UFS/FFS is supported as it's the default fs
   on OpenBSD systems.

   See: https://man.openbsd.org/mount.2
*/

package mount

/*
#include <sys/types.h>
#include <sys/mount.h>
*/
import "C"

import (
	"fmt"
	"syscall"
	"unsafe"
)

func createExportInfo(readOnly bool) C.struct_export_args {
	exportFlags := C.int(0)
	if readOnly {
		exportFlags = C.MNT_EXRDONLY
	}
	out := C.struct_export_args{
		ex_root:  0,
		ex_flags: exportFlags,
	}
	return out
}

func createUfsArgs(device string, readOnly bool) unsafe.Pointer {
	out := &C.struct_ufs_args{
		fspec:       C.CString(device),
		export_info: createExportInfo(readOnly),
	}
	return unsafe.Pointer(out)
}

func mount(device, target, mType string, flag uintptr, data string) error {
	readOnly := flag&RDONLY != 0

	var fsArgs unsafe.Pointer

	switch mType {
	case "ffs":
		fsArgs = createUfsArgs(device, readOnly)
	default:
		return &mountError{
			op:     "mount",
			source: device,
			target: target,
			flags:  flag,
			err:    fmt.Errorf("unsupported file system type: %s", mType),
		}
	}

	if errno := C.mount(C.CString(mType), C.CString(target), C.int(flag), fsArgs); errno != 0 {
		return &mountError{
			op:     "mount",
			source: device,
			target: target,
			flags:  flag,
			err:    syscall.Errno(errno),
		}
	}

	return nil
}
