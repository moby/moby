package build

// CacheDiskUsage contains disk usage for the build cache.
//
// Deprecated: this type is no longer used and will be removed in the next release.
type CacheDiskUsage struct {
	TotalSize   int64
	Reclaimable int64
	Items       []*CacheRecord
}
