package client

import (
	"github.com/moby/moby/api/types/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImageHistoryOption is a type representing functional options for the image history operation.
type ImageHistoryOption interface {
	Apply(*imageHistoryOpts) error
}
type imageHistoryOptionFunc func(opt *imageHistoryOpts) error

func (f imageHistoryOptionFunc) Apply(o *imageHistoryOpts) error {
	return f(o)
}

type imageHistoryOpts struct {
	apiOptions imageHistoryOptions
}

type imageHistoryOptions struct {
	// Platform from the manifest list to use for history.
	Platform *ocispec.Platform
}

type ImageHistoryResult struct {
	Items []image.HistoryResponseItem
}
