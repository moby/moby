package container

import "github.com/moby/moby/api/types/filters"

// ListOptions holds parameters to list containers with.
type ListOptions struct {
	Size    bool
	All     bool
	Latest  bool
	Since   string
	Before  string
	Limit   int
	Filters filters.Args
}
