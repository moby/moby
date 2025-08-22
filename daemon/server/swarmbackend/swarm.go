package swarmbackend

import "github.com/moby/moby/api/types/filters"

type ConfigListOptions struct {
	Filters filters.Args
}

type NodeListOptions struct {
	Filters filters.Args
}

type TaskListOptions struct {
	Filters filters.Args
}
