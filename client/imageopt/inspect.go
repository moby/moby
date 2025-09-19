package imageopt

import (
	"bytes"
	"context"
	"fmt"

	"github.com/moby/moby/client"
	"github.com/moby/moby/client/internal/opts"
)

func (o *withSinglePlatformOption) ApplyImageInspectOption(ctx context.Context, opts *opts.ImageInspectOptions) error {
	if opts.ApiOptions.Platform != nil {
		return fmt.Errorf("platform already set to %v", opts.ApiOptions.Platform)
	}

	opts.ApiOptions.Platform = &o.platform
	return nil
}

// WithRawResponse instructs the client to additionally store the
// raw inspect response in the provided buffer.
func WithRawResponse(raw *bytes.Buffer) client.ImageInspectOption {
	return opts.InspectOptionFunc(func(_ context.Context, opts *opts.ImageInspectOptions) error {
		opts.Raw = raw
		return nil
	})
}

// WithImageManifests sets manifests API option for the image inspect operation.
// This option is only available for API version 1.48 and up.
// With this option set, the image inspect operation response includes
// the [image.InspectResponse.Manifests] field if the server is multi-platform
// capable.
func WithImageManifests(manifests bool) client.ImageInspectOption {
	return opts.InspectOptionFunc(func(_ context.Context, clientOpts *opts.ImageInspectOptions) error {
		clientOpts.ApiOptions.Manifests = manifests
		return nil
	})
}
