// +build linux,cgo,!static_build
// +build libdm_dlsym_deferred_remove,!libdm_no_deferred_remove

package devicemapper

/*
#cgo LDFLAGS: -ldl
#include <stdlib.h>
#include <dlfcn.h>
#include <libdevmapper.h>

// Yes, I know this looks scary. In order to be able to fill our own internal
// dm_info with deferred_remove we need to have a struct definition that is
// correct (regardless of the version of libdm that was used to compile it). To
// this end, we define struct_backport_dm_info. This code comes from lvm2, and
// I have verified that the structure has only ever had elements *appended* to
// it (since 2001).
//
// It is also important that this structure be _larger_ than the dm_info that
// libdevmapper expected. Otherwise libdm might try to write to memory it
// shouldn't (they don't have a "known size" API).
struct backport_dm_info {
	int exists;
	int suspended;
	int live_table;
	int inactive_table;
	int32_t open_count;
	uint32_t event_nr;
	uint32_t major;
	uint32_t minor;
	int read_only;

	int32_t target_count;

	int deferred_remove;
	int internal_suspend;

	// Padding, purely for our own safety. This is to avoid cases where libdm
	// was updated underneath us and we call into dm_task_get_info() with too
	// small of a buffer.
	char _[512];
};

// We have to wrap this in CGo, because Go really doesn't like function pointers.
int call_dm_task_deferred_remove(void *fn, struct dm_task *task)
{
	int (*_dm_task_deferred_remove)(struct dm_task *task) = fn;
	return _dm_task_deferred_remove(task);
}
*/
import "C"

import (
	"unsafe"

	"github.com/sirupsen/logrus"
)

// dm_task_deferred_remove is not supported by all distributions, due to
// out-dated versions of devicemapper. However, in the case where the
// devicemapper library was updated without rebuilding Docker (which can happen
// in some distributions) then we should attempt to dynamically load the
// relevant object rather than try to link to it.

// dmTaskDeferredRemoveFct is a "bound" version of dm_task_deferred_remove.
// It is nil if dm_task_deferred_remove was not found in the libdevmapper that
// is currently loaded.
var dmTaskDeferredRemovePtr unsafe.Pointer

// LibraryDeferredRemovalSupport tells if the feature is supported by the
// current Docker invocation. This value is fixed during init.
var LibraryDeferredRemovalSupport bool

func init() {
	// Clear any errors.
	var err *C.char
	C.dlerror()

	// The symbol we want to fetch.
	symName := C.CString("dm_task_deferred_remove")
	defer C.free(unsafe.Pointer(symName))

	// See if we can find dm_task_deferred_remove. Since we already are linked
	// to libdevmapper, we can search our own address space (rather than trying
	// to guess what libdevmapper is called). We use NULL here, as RTLD_DEFAULT
	// is not available in CGO (even if you set _GNU_SOURCE for some reason).
	// The semantics are identical on glibc.
	sym := C.dlsym(nil, symName)
	err = C.dlerror()
	if err != nil {
		logrus.Debugf("devmapper: could not load dm_task_deferred_remove: %s", C.GoString(err))
		return
	}

	logrus.Debugf("devmapper: found dm_task_deferred_remove at %x", uintptr(sym))
	dmTaskDeferredRemovePtr = sym
	LibraryDeferredRemovalSupport = true
}

func dmTaskDeferredRemoveFct(task *cdmTask) int {
	sym := dmTaskDeferredRemovePtr
	if sym == nil || !LibraryDeferredRemovalSupport {
		return -1
	}
	return int(C.call_dm_task_deferred_remove(sym, (*C.struct_dm_task)(task)))
}

func dmTaskGetInfoWithDeferredFct(task *cdmTask, info *Info) int {
	if !LibraryDeferredRemovalSupport {
		return -1
	}

	Cinfo := C.struct_backport_dm_info{}
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
	return int(C.dm_task_get_info((*C.struct_dm_task)(task), (*C.struct_dm_info)(unsafe.Pointer(&Cinfo))))
}
