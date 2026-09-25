package client

import (
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type ImageSaveOption interface {
	Apply(*imageSaveOpts) error
}

type imageSaveOptionFunc func(opt *imageSaveOpts) error

func (f imageSaveOptionFunc) Apply(o *imageSaveOpts) error {
	return f(o)
}

// ImageSaveWithPlatforms sets the platforms to be saved from the image. It
// produces an error if platforms are already set. This option only has an
// effect if the input image is a multi-platform image.
func ImageSaveWithPlatforms(platforms ...ocispec.Platform) ImageSaveOption {
	// TODO(thaJeztah): verify the GoDoc; do we produce an error for a single-platform image without the given platform?
	return imageSaveOptionFunc(func(opt *imageSaveOpts) error {
		if opt.apiOptions.Platforms != nil {
			return fmt.Errorf("platforms already set to %v", opt.apiOptions.Platforms)
		}
		opt.apiOptions.Platforms = platforms
		return nil
	})
}

type imageSaveOpts struct {
	apiOptions imageSaveOptions
}

type imageSaveOptions struct {
	// Platforms selects the platforms to save if the image is a
	// multi-platform image and has multiple variants.
	Platforms []ocispec.Platform
}
