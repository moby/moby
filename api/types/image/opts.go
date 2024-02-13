package image

import "github.com/docker/docker/api/types/filters"

// ImportOptions holds information to import images from the client host.
type ImportOptions struct {
	Tag      string   // Tag is the name to tag this image with. This attribute is deprecated.
	Message  string   // Message is the message to tag the image with
	Changes  []string // Changes are the raw changes to apply to this image
	Platform string   // Platform is the target platform of the image
}

// CreateOptions holds information to create images.
type CreateOptions struct {
	RegistryAuth string // RegistryAuth is the base64 encoded credentials for the registry.
	Platform     string // Platform is the target platform of the image if it needs to be pulled from the registry.
}

// PullOptions holds information to pull images.
type PullOptions struct {
	All          bool
	RegistryAuth string // RegistryAuth is the base64 encoded credentials for the registry

	// PrivilegeFunc is a function that clients can supply to retry operations
	// after getting an authorization error. This function returns the registry
	// authentication header value in base64 encoded format, or an error if the
	// privilege request fails.
	//
	// Also see [github.com/docker/docker/api/types.RequestPrivilegeFunc].
	PrivilegeFunc func() (string, error)
	Platform      string
}

// PushOptions holds information to push images.
type PushOptions PullOptions

// ListOptions holds parameters to list images with.
type ListOptions struct {
	// All controls whether all images in the graph are filtered, or just
	// the heads.
	All bool

	// Filters is a JSON-encoded set of filter arguments.
	Filters filters.Args

	// SharedSize indicates whether the shared size of images should be computed.
	SharedSize bool

	// ContainerCount indicates whether container count should be computed.
	ContainerCount bool
}

// RemoveOptions holds parameters to remove images.
type RemoveOptions struct {
	Force         bool
	PruneChildren bool
}
