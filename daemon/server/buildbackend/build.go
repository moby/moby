package buildbackend

import "github.com/moby/moby/api/types/filters"

type CachePruneOptions struct {
	All           bool
	ReservedSpace int64
	MaxUsedSpace  int64
	MinFreeSpace  int64
	Filters       filters.Args

	KeepStorage int64 // Deprecated: deprecated in API 1.48.
}
