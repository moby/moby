package volume

// DiskUsage contains disk usage for volumes.
//
// Deprecated: this type is no longer used and will be removed in the next release.
type DiskUsage struct {
	TotalSize   int64
	Reclaimable int64
	Items       []*Volume
}
