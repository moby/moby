//go:build linux && cgo && !static_build && !libdm_dlsym_deferred_remove && !libdm_no_deferred_remove
// +build linux,cgo,!static_build,!libdm_dlsym_deferred_remove,!libdm_no_deferred_remove

package devicemapper // import "github.com/docker/docker/pkg/devicemapper"

/*
#include <libdevmapper.h>
*/
import "C"

// LibraryDeferredRemovalSupport tells if the feature is supported by the
// current Docker invocation.
const LibraryDeferredRemovalSupport = true

func dmTaskDeferredRemoveFct(task *cdmTask) int {
	return int(C.dm_task_deferred_remove((*C.struct_dm_task)(task)))
}

func dmTaskGetInfoWithDeferredFct(task *cdmTask, info *Info) int {
	Cinfo := C.struct_dm_info{}
	defer func() {
		info.Exists = int(Cinfo.exists)
		info.Suspended = int(Cinfo.suspended)
		info.LiveTable = int(Cinfo.live_table)
		info.InactiveTable = int(Cinfo.inactive_table)
		info.OpenCount = int32(Cinfo.open_count)
		info.EventNr = uint32(Cinfo.event_nr)
		info.Major = uint32(Cinfo.major)
		info.Minor = uint32(Cinfo.minor)
		info.ReadOnly = int(Cinfo.read_only)
		info.TargetCount = int32(Cinfo.target_count)
		info.DeferredRemove = int(Cinfo.deferred_remove)
	}()
	return int(C.dm_task_get_info((*C.struct_dm_task)(task), &Cinfo))
}
