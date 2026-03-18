package build

import (
	"time"
)

// CacheRecord contains information about a build cache record.
type CacheRecord struct {
	// ID is the unique ID of the build cache record.
	ID string
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

// CachePruneReport contains the response for Engine API:
// POST "/build/prune"
type CachePruneReport struct {
	CachesDeleted  []string
	SpaceReclaimed uint64
}
