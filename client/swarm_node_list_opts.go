package client

import "github.com/moby/moby/api/types/filters"

// NodeListOptions holds parameters to list nodes with.
type NodeListOptions struct {
	Filters filters.Args
}
