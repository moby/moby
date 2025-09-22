package imagehistory

import (
	"fmt"

	"github.com/moby/moby/client/internal/opts"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type Option = opts.Option[opts.ImageHistoryOptions]

// WithPlatform selects the specific platform of a multi-platform image.  This
// effectively causes the operation to work on a single-platform image manifest
// dereferenced from the original OCI index using the provided platform.
//
// Minimum API version required: 1.48
func WithPlatform(platform ocispec.Platform) Option {
	return opts.OptionFunc[opts.ImageHistoryOptions](func(opts *opts.ImageHistoryOptions) error {
		if opts.ApiOptions.Platform != nil {
			return fmt.Errorf("platform already set to %v", opts.ApiOptions.Platform)
		}

		opts.ApiOptions.Platform = &platform
		return nil
	})
}
