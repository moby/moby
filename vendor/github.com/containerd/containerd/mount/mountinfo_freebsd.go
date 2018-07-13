/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package mount

/*
#include <sys/param.h>
#include <sys/ucred.h>
#include <sys/mount.h>
*/
import "C"

import (
	"fmt"
	"reflect"
	"unsafe"
)

// Self retrieves a list of mounts for the current running process.
func Self() ([]Info, error) {
	var rawEntries *C.struct_statfs

	count := int(C.getmntinfo(&rawEntries, C.MNT_WAIT))
	if count == 0 {
		return nil, fmt.Errorf("Failed to call getmntinfo")
	}

	var entries []C.struct_statfs
	header := (*reflect.SliceHeader)(unsafe.Pointer(&entries))
	header.Cap = count
	header.Len = count
	header.Data = uintptr(unsafe.Pointer(rawEntries))

	var out []Info
	for _, entry := range entries {
		var mountinfo Info
		mountinfo.Mountpoint = C.GoString(&entry.f_mntonname[0])
		mountinfo.Source = C.GoString(&entry.f_mntfromname[0])
		mountinfo.FSType = C.GoString(&entry.f_fstypename[0])
		out = append(out, mountinfo)
	}
	return out, nil
}

// PID collects the mounts for a specific process ID.
func PID(pid int) ([]Info, error) {
	return nil, fmt.Errorf("mountinfo.PID is not implemented on freebsd")
}
