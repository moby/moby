// +build linux,cgo
// +build !libdm_dlsym_deferred_remove,libdm_no_deferred_remove

package devicemapper // import "github.com/docker/docker/pkg/devicemapper"

// LibraryDeferredRemovalSupport tells if the feature is supported by the
// current Docker invocation.
const LibraryDeferredRemovalSupport = false

func dmTaskDeferredRemoveFct(task *cdmTask) int {
	// Error. Nobody should be calling it.
	return -1
}

func dmTaskGetInfoWithDeferredFct(task *cdmTask, info *Info) int {
	return -1
}
