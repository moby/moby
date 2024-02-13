package dockerfile // import "github.com/docker/docker/builder/dockerfile"

import (
	"context"

	"github.com/containerd/log"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImageProber exposes an Image cache to the Builder. It supports resetting a
// cache.
type ImageProber interface {
	Reset(ctx context.Context) error
	Probe(parentID string, runConfig *container.Config, platform ocispec.Platform) (string, error)
}

type resetFunc func(context.Context) (builder.ImageCache, error)

type imageProber struct {
	cache       builder.ImageCache
	reset       resetFunc
	cacheBusted bool
}

func newImageProber(ctx context.Context, cacheBuilder builder.ImageCacheBuilder, cacheFrom []string, noCache bool) (ImageProber, error) {
	if noCache {
		return &nopProber{}, nil
	}

	reset := func(ctx context.Context) (builder.ImageCache, error) {
		return cacheBuilder.MakeImageCache(ctx, cacheFrom)
	}

	cache, err := reset(ctx)
	if err != nil {
		return nil, err
	}
	return &imageProber{cache: cache, reset: reset}, nil
}

func (c *imageProber) Reset(ctx context.Context) error {
	newCache, err := c.reset(ctx)
	if err != nil {
		return err
	}
	c.cache = newCache
	c.cacheBusted = false
	return nil
}

// Probe checks if cache match can be found for current build instruction.
// It returns the cachedID if there is a hit, and the empty string on miss
func (c *imageProber) Probe(parentID string, runConfig *container.Config, platform ocispec.Platform) (string, error) {
	if c.cacheBusted {
		return "", nil
	}
	cacheID, err := c.cache.GetCache(parentID, runConfig, platform)
	if err != nil {
		return "", err
	}
	if len(cacheID) == 0 {
		log.G(context.TODO()).Debugf("[BUILDER] Cache miss: %s", runConfig.Cmd)
		c.cacheBusted = true
		return "", nil
	}
	log.G(context.TODO()).Debugf("[BUILDER] Use cached version: %s", runConfig.Cmd)
	return cacheID, nil
}

type nopProber struct{}

func (c *nopProber) Reset(ctx context.Context) error {
	return nil
}

func (c *nopProber) Probe(_ string, _ *container.Config, _ ocispec.Platform) (string, error) {
	return "", nil
}
