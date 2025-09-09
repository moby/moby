package client

import "github.com/moby/moby/api/types/filters"

// NetworkListOptions holds parameters to filter the list of networks with.
type NetworkListOptions struct {
	Filters filters.Args
}
