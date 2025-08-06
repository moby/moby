package client

import "github.com/moby/moby/api/types/filters"

// ConfigListOptions holds parameters to list configs
type ConfigListOptions struct {
	Filters filters.Args
}
