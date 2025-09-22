package imageinspect

import (
	"bytes"
	"fmt"

	"github.com/moby/moby/client/internal/opts"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type Option = opts.Option[opts.ImageInspectOptions]

// WithRawResponse instructs the client to additionally store the
// raw inspect response in the provided buffer.
func WithRawResponse(raw *bytes.Buffer) Option {
	return opts.OptionFunc[opts.ImageInspectOptions](func(opts *opts.ImageInspectOptions) error {
		opts.Raw = raw
		return nil
	})
}

// WithImageManifests sets manifests API option for the image inspect operation.
// This option is only available for API version 1.48 and up.
// With this option set, the image inspect operation response includes
// the [image.InspectResponse.Manifests] field if the server is multi-platform
// capable.
func WithImageManifests(manifests bool) Option {
	return opts.OptionFunc[opts.ImageInspectOptions](func(clientOpts *opts.ImageInspectOptions) error {
		clientOpts.ApiOptions.Manifests = manifests
		return nil
	})
}

// WithPlatform selects the specific platform of a multi-platform image.  This
// effectively causes the operation to work on a single-platform image manifest
// dereferenced from the original OCI index using the provided platform.
//
// Minimum API version required: 1.49
func WithPlatform(platform ocispec.Platform) Option {
	return opts.OptionFunc[opts.ImageInspectOptions](func(opts *opts.ImageInspectOptions) error {
		if opts.ApiOptions.Platform != nil {
			return fmt.Errorf("platform already set to %v", opts.ApiOptions.Platform)
		}

		opts.ApiOptions.Platform = &platform
		return nil
	})
}
