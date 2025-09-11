package imageopt

import (
	"context"
	"fmt"

	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/moby/moby/client"
	"github.com/moby/moby/client/internal/opts"
)

func WithPlatformString(platform string) client.ImageInspectOption {
	return opts.InspectOptionFunc(func(ctx context.Context, opts *opts.ImageInspectOptions) error {
		p, err := platforms.Parse(platform)
		if err != nil {
			return fmt.Errorf("failed to parse platform: %w", err)
		}
		return WithPlatform(p).ApplyImageInspectOption(ctx, opts)
	})
}

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
