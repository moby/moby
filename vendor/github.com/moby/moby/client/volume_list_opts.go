package client

import "github.com/moby/moby/api/types/filters"

// ListOptions holds parameters to list volumes.
type ListOptions struct {
	Filters filters.Args
}
