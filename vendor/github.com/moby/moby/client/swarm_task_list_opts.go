package client

import "github.com/moby/moby/api/types/filters"

// TaskListOptions holds parameters to list tasks with.
type TaskListOptions struct {
	Filters filters.Args
}
