package client

import "github.com/moby/moby/api/types/filters"

// ServiceListOptions holds parameters to list services with.
type ServiceListOptions struct {
	Filters filters.Args

	// Status indicates whether the server should include the service task
	// count of running and desired tasks.
	Status bool
}
