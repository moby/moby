package client

import (
	"bytes"
	"context"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImageInspectOption is a type representing functional options for the image inspect operation.
type ImageInspectOption interface {
	applyImageInspectOption(ctx context.Context, opts *imageInspectOpts) error
}
type imageInspectOptionFunc func(opt *imageInspectOpts) error

func (f imageInspectOptionFunc) applyImageInspectOption(ctx context.Context, o *imageInspectOpts) error {
	return f(o)
}

// WithPlatform selects the specific platform of a multi-platform image.  This
// effectively causes the operation to work on a single-platform image manifest
// dereferenced from the original OCI index using the provided platform.
//
// Minimum API version: 1.49
func WithInspectPlatform(platform ocispec.Platform) ImageInspectOption {
	return imageInspectOptionFunc(func(opts *imageInspectOpts) error {
		opts.apiOptions.Platform = &platform
		return nil
	})
}

// WithRawResponse instructs the client to additionally store the
// raw inspect response in the provided buffer.
func WithRawResponse(raw *bytes.Buffer) ImageInspectOption {
	return imageInspectOptionFunc(func(opts *imageInspectOpts) error {
		opts.raw = raw
		return nil
	})
}

// WithImageManifests sets manifests API option for the image inspect operation.
// This option is only available for API version 1.48 and up.
// With this option set, the image inspect operation response includes
// the [image.InspectResponse.Manifests] field if the server is multi-platform
// capable.
func WithImageManifests(manifests bool) ImageInspectOption {
	return imageInspectOptionFunc(func(clientOpts *imageInspectOpts) error {
		clientOpts.apiOptions.Manifests = manifests
		return nil
	})
}

type imageInspectOpts struct {
	raw        *bytes.Buffer
	apiOptions imageInspectOptions
}

type imageInspectOptions struct {
	// Manifests returns the image manifests.
	Manifests bool

	// Platform selects the specific platform of a multi-platform image to inspect.
	//
	// This option is only available for API version 1.49 and up.
	Platform *ocispec.Platform
}
