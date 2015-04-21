// +build linux,!libdm_no_deferred_remove

package devicemapper

/*
#cgo LDFLAGS: -L. -ldevmapper
#include <libdevmapper.h>
*/
import "C"

const LibraryDeferredRemovalSupport = true

func dmTaskDeferredRemoveFct(task *CDmTask) int {
	return int(C.dm_task_deferred_remove((*C.struct_dm_task)(task)))
}
