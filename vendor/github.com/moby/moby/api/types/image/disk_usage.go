package image

// DiskUsage contains disk usage for images.
type DiskUsage struct {
	TotalSize   int64
	Reclaimable int64
	Items       []*Summary
}
