package volumebackend

import "github.com/moby/moby/api/types/filters"

type ListOptions struct {
	Filters filters.Args
}
