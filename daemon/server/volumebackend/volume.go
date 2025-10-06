package volumebackend

import "github.com/moby/moby/v2/daemon/internal/filters"

type ListOptions struct {
	Filters filters.Args
}
