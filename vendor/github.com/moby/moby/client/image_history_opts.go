package client

import (
	"context"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type ImageHistoryOption interface {
	applyImageHistoryOption(ctx context.Context, opts *imageHistoryOpts) error
}

type imageHistoryOpts struct {
	apiOptions imageHistoryOptions
}

type imageHistoryOptions struct {
	// Platform from the manifest list to use for history.
	Platform *ocispec.Platform
}
