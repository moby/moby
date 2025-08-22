package client

import "github.com/moby/moby/api/types/filters"

// ListOptions holds parameters to filter the list of networks with.
type ListOptions struct {
	Filters filters.Args
}
