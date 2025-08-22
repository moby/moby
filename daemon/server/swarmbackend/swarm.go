package swarmbackend

import "github.com/moby/moby/api/types/filters"

type ConfigListOptions struct {
	Filters filters.Args
}
