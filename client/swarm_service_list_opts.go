package client

// ServiceListOptions holds parameters to list services with.
type ServiceListOptions struct {
	Filters Filters

	// Status indicates whether the server should include the service task
	// count of running and desired tasks.
	Status bool
}
