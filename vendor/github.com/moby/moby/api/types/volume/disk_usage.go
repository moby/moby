package volume

// DiskUsage contains disk usage for volumes.
type DiskUsage struct {
	TotalSize   int64
	Reclaimable int64
	Items       []*Volume
}
