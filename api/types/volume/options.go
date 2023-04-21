package volume // import "github.com/docker/docker/api/types/volume"

import "github.com/docker/docker/api/types/filters"

// ListOptions holds parameters to list volumes.
type ListOptions struct {
	Filters filters.Args
}

// PruneOptions holds parameters to prune volumes.
type PruneOptions struct {
	// All controls whether named volumes should also be pruned.
	All bool

	// Filters to apply when pruning.
	Filters filters.Args
}

// PruneReport contains the response for Engine API:
// POST "/volumes/prune"
type PruneReport struct {
	VolumesDeleted []string
	SpaceReclaimed uint64
}
