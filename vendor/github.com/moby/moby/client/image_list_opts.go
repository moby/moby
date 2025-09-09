package client

import "github.com/moby/moby/api/types/filters"

// ImageListOption is a type representing functional options for the image list operation.
type ImageListOption interface {
	ApplyImageListOption(*imageListOpts) error
}

type imageListOptionFunc func(opt *imageListOpts) error

func (f imageListOptionFunc) ApplyImageListOption(o *imageListOpts) error {
	return f(o)
}

// ImageListWithAll controls whether all images in the graph are filtered, or just the heads.
func ImageListWithAll(all bool) ImageListOption {
	return imageListOptionFunc(func(opts *imageListOpts) error {
		opts.apiOptions.All = all
		return nil
	})
}

// ImageListWithFilters sets the filters for the image list operation.
// Filters is a JSON-encoded set of filter arguments.
func ImageListWithFilters(filters filters.Args) ImageListOption {
	return imageListOptionFunc(func(opts *imageListOpts) error {
		opts.apiOptions.Filters = filters
		return nil
	})
}

// ImageListWithSharedSize indicates whether the shared size of images should be computed.
func ImageListWithSharedSize(sharedSize bool) ImageListOption {
	return imageListOptionFunc(func(opts *imageListOpts) error {
		opts.apiOptions.SharedSize = sharedSize
		return nil
	})
}

// ImageListWithManifests indicates whether the image manifests should be returned.
func ImageListWithManifests(manifests bool) ImageListOption {
	return imageListOptionFunc(func(opts *imageListOpts) error {
		opts.apiOptions.Manifests = manifests
		return nil
	})
}

type imageListOpts struct {
	apiOptions imageListOptions
}

type imageListOptions struct {
	// All controls whether all images in the graph are filtered, or just
	// the heads.
	All bool

	// Filters is a JSON-encoded set of filter arguments.
	Filters filters.Args

	// SharedSize indicates whether the shared size of images should be computed.
	SharedSize bool

	// Manifests indicates whether the image manifests should be returned.
	Manifests bool
}
