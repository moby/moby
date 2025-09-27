package imageopt

import (
	"context"
	"fmt"

	"github.com/containerd/platforms"
	"github.com/moby/moby/client/internal/opts"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type platformString struct {
	platform string
}

func WithPlatformString(platform string) *platformString {
	return &platformString{platform: platform}
}

func parsePlatform(platform string) (ocispec.Platform, error) {
	p, err := platforms.Parse(platform)
	if err != nil {
		return ocispec.Platform{}, err
	}
	return p, nil
}

func (o *platformString) ApplyImageHistoryOption(ctx context.Context, opts *opts.ImageHistoryOptions) error {
	if opts.ApiOptions.Platform != nil {
		return fmt.Errorf("platform already set to %v", opts.ApiOptions.Platform)
	}

	p, err := parsePlatform(o.platform)
	if err != nil {
		return err
	}

	opts.ApiOptions.Platform = &p
	return nil
}

func (o *platformString) ApplyImageInspectOption(ctx context.Context, opts *opts.ImageInspectOptions) error {
	if opts.ApiOptions.Platform != nil {
		return fmt.Errorf("platform already set to %v", opts.ApiOptions.Platform)
	}

	p, err := parsePlatform(o.platform)
	if err != nil {
		return err
	}

	opts.ApiOptions.Platform = &p
	return nil
}
