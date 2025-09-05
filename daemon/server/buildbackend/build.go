package buildbackend

import "github.com/moby/moby/api/types/filters"

type CachePruneOptions struct {
	All           bool
	ReservedSpace int64
	MaxUsedSpace  int64
	MinFreeSpace  int64
	Filters       filters.Args
}
