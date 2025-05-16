package build

import (
	"time"

	"github.com/docker/docker/api/types/filters"
)

// CacheRecord contains information about a build cache record.
type CacheRecord struct {
	// ID is the unique ID of the build cache record.
	ID string
	// Parent is the ID of the parent build cache record.
	//
	// Deprecated: deprecated in API v1.42 and up, as it was deprecated in BuildKit; use Parents instead.
	Parent string `json:"Parent,omitempty"`
	// Parents is the list of parent build cache record IDs.
	Parents []string `json:" Parents,omitempty"`
	// Type is the cache record type.
	Type string
	// Description is a description of the build-step that produced the build cache.
	Description string
	// InUse indicates if the build cache is in use.
	InUse bool
	// Shared indicates if the build cache is shared.
	Shared bool
	// Size is the amount of disk space used by the build cache (in bytes).
	Size int64
	// CreatedAt is the date and time at which the build cache was created.
	CreatedAt time.Time
	// LastUsedAt is the date and time at which the build cache was last used.
	LastUsedAt *time.Time
	UsageCount int
}

// CachePruneOptions hold parameters to prune the build cache.
type CachePruneOptions struct {
	All           bool
	ReservedSpace int64
	MaxUsedSpace  int64
	MinFreeSpace  int64
	Filters       filters.Args

	KeepStorage int64 // Deprecated: deprecated in API 1.48.
}

// CachePruneReport contains the response for Engine API:
// POST "/build/prune"
type CachePruneReport struct {
	CachesDeleted  []string
	SpaceReclaimed uint64
}
