package opts

import (
	"bytes"
	"context"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type InspectOptionFunc func(ctx context.Context, opt *ImageInspectOptions) error

func (f InspectOptionFunc) ApplyImageInspectOption(ctx context.Context, o *ImageInspectOptions) error {
	return f(ctx, o)
}

type ImageInspectOptions struct {
	Raw        *bytes.Buffer
	ApiOptions ImageInspectApiOptions
}

type ImageInspectApiOptions struct {
	// Manifests returns the image manifests.
	Manifests bool

	// Platform selects the specific platform of a multi-platform image to inspect.
	//
	// This option is only available for API version 1.49 and up.
	Platform *ocispec.Platform
}
