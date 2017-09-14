// +build solaris,cgo

package mount

/*
#include <stdio.h>
#include <sys/mnttab.h>
*/
import "C"

import (
	"fmt"
)

// Self retrieves a list of mounts for the current running process.
func Self() ([]Info, error) {
	mnttab := C.fopen(C.CString(C.MNTTAB), C.CString("r"))
	if mnttab == nil {
		return nil, fmt.Errorf("Failed to open %s", C.MNTTAB)
	}

	var out []Info
	var mp C.struct_mnttab

	ret := C.getmntent(mnttab, &mp)
	for ret == 0 {
		var mountinfo Info
		mountinfo.Mountpoint = C.GoString(mp.mnt_mountp)
		mountinfo.Source = C.GoString(mp.mnt_special)
		mountinfo.FSType = C.GoString(mp.mnt_fstype)
		mountinfo.Options = C.GoString(mp.mnt_mntopts)
		out = append(out, mountinfo)
		ret = C.getmntent(mnttab, &mp)
	}

	C.fclose(mnttab)
	return out, nil
}

// PID collects the mounts for a specific process ID.
func PID(pid int) ([]Info, error) {
	return nil, fmt.Errorf("mountinfo.PID is not implemented on solaris")
}
