package container

// DiskUsage contains disk usage for containers.
type DiskUsage struct {
	TotalSize   int64
	Reclaimable int64
	Items       []*Summary
}
