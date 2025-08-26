package client

import "github.com/moby/moby/api/types/filters"

// ImageListOptions holds parameters to list images with.
type ImageListOptions struct {
	// All controls whether all images in the graph are filtered, or just
	// the heads.
	All bool

	// Filters is a JSON-encoded set of filter arguments.
	Filters filters.Args

	// SharedSize indicates whether the shared size of images should be computed.
	SharedSize bool

	// ContainerCount indicates whether container count should be computed.
	//
	// Deprecated: This field has been unused and is no longer required and will be removed in a future version.
	ContainerCount bool

	// Manifests indicates whether the image manifests should be returned.
	Manifests bool
}
