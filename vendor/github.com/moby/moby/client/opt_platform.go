package client

import (
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type withSinglePlatformOption struct {
	platform ocispec.Platform
}

// WithPlatform selects the specific platform of a multi-platform image.  This
// effectively causes the operation to work on a single-platform image manifest
// dereferenced from the original OCI index using the provided platform.
//
// Minimum API version requirements:
//
// - ImageInspect: 1.49
//
// - ImageHistory: 1.48
func WithPlatform(platform ocispec.Platform) *withSinglePlatformOption {
	return &withSinglePlatformOption{platform: platform}
}

func (o *withSinglePlatformOption) ApplyImageHistoryOption(opts *imageHistoryOpts) error {
	if opts.apiOptions.Platform != nil {
		return fmt.Errorf("platform already set to %v", opts.apiOptions.Platform)
	}

	opts.apiOptions.Platform = &o.platform
	return nil
}

func (o *withSinglePlatformOption) ApplyImageInspectOption(opts *imageInspectOpts) error {
	if opts.apiOptions.Platform != nil {
		return fmt.Errorf("platform already set to %v", opts.apiOptions.Platform)
	}

	opts.apiOptions.Platform = &o.platform
	return nil
}
