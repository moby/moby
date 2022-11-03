package dockerfile // import "github.com/docker/docker/builder/dockerfile"

import (
	"context"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder"
	"github.com/sirupsen/logrus"
)

// ImageProber exposes an Image cache to the Builder. It supports resetting a
// cache.
type ImageProber interface {
	Reset(ctx context.Context) error
	Probe(parentID string, runConfig *container.Config) (string, error)
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
func (c *imageProber) Probe(parentID string, runConfig *container.Config) (string, error) {
	if c.cacheBusted {
		return "", nil
	}
	cacheID, err := c.cache.GetCache(parentID, runConfig)
	if err != nil {
		return "", err
	}
	if len(cacheID) == 0 {
		logrus.Debugf("[BUILDER] Cache miss: %s", runConfig.Cmd)
		c.cacheBusted = true
		return "", nil
	}
	logrus.Debugf("[BUILDER] Use cached version: %s", runConfig.Cmd)
	return cacheID, nil
}

type nopProber struct{}

func (c *nopProber) Reset(ctx context.Context) error {
	return nil
}

func (c *nopProber) Probe(_ string, _ *container.Config) (string, error) {
	return "", nil
}
