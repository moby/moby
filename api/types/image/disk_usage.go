package image

// DiskUsage contains disk usage for images.
//
// Deprecated: this type is no longer used and will be removed in the next release.
type DiskUsage struct {
	TotalSize   int64
	Reclaimable int64
	Items       []*Summary
}
