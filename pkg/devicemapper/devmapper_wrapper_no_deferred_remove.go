// +build linux,libdm_no_deferred_remove

package devicemapper

const LibraryDeferredRemovalSupport = false

func dmTaskDeferredRemoveFct(task *CDmTask) int {
	// Error. Nobody should be calling it.
	return -1
}
