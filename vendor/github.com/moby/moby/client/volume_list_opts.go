package client

import "github.com/moby/moby/api/types/filters"

// VolumeListOptions holds parameters to list volumes.
type VolumeListOptions struct {
	Filters filters.Args
}
