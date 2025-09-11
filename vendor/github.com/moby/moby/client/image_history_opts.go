package client

import (
	"context"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type ImageHistoryOption interface {
	applyImageHistoryOption(ctx context.Context, opts *imageHistoryOpts) error
}

type imageHistoryOptionFunc func(opt *imageHistoryOpts) error

func (f imageHistoryOptionFunc) applyImageHistoryOption(ctx context.Context, o *imageHistoryOpts) error {
	return f(o)
}

type imageHistoryOpts struct {
	apiOptions imageHistoryOptions
}

type imageHistoryOptions struct {
	// Platform from the manifest list to use for history.
	Platform *ocispec.Platform
}

// WithPlatform selects the specific platform of a multi-platform image.  This
// effectively causes the operation to work on a single-platform image manifest
// dereferenced from the original OCI index using the provided platform.
//
// Minimum API version: 1.48
func WithHistoryPlatform(platform ocispec.Platform) ImageHistoryOption {
	return imageHistoryOptionFunc(func(opts *imageHistoryOpts) error {
		opts.apiOptions.Platform = &platform
		return nil
	})
}
