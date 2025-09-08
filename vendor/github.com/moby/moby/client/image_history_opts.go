package client

import ocispec "github.com/opencontainers/image-spec/specs-go/v1"

type ImageHistoryOption interface {
	ApplyImageHistoryOption(*imageHistoryOpts) error
}
type imageHistoryOpts struct {
	apiOptions imageHistoryOptions
}

type imageHistoryOptions struct {
	// Platform from the manifest list to use for history.
	Platform *ocispec.Platform
}
