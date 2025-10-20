package client

import "github.com/moby/moby/api/types/image"

// ImageListOptions holds parameters to list images with.
type ImageListOptions struct {
	// All controls whether all images in the graph are filtered, or just
	// the heads.
	All bool

	// Filters is a JSON-encoded set of filter arguments.
	Filters Filters

	// SharedSize indicates whether the shared size of images should be computed.
	SharedSize bool

	// Manifests indicates whether the image manifests should be returned.
	Manifests bool
}

// ImageListResult holds the result from ImageList.
type ImageListResult struct {
	Items []image.Summary
}
