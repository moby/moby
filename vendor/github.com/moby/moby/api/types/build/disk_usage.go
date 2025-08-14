package build

// CacheDiskUsage contains disk usage for the build cache.
type CacheDiskUsage struct {
	TotalSize   int64
	Reclaimable int64
	Items       []*CacheRecord
}
