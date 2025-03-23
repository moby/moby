package cache

import (
	"context"
	"errors"

	"github.com/containerd/log"
	"github.com/docker/distribution"
	prometheus "github.com/docker/distribution/metrics"
	"github.com/opencontainers/go-digest"
)

// Logger can be provided on the MetricsTracker to log errors.
//
// Usually, this is just a proxy to dcontext.GetLogger.
type Logger interface {
	Errorf(format string, args ...any)
}

type cachedBlobStatter struct {
	cache   distribution.BlobDescriptorService
	backend distribution.BlobDescriptorService
}

// cacheCount is the number of total cache request received/hits/misses
var cacheCount = prometheus.StorageNamespace.NewLabeledCounter("cache", "The number of cache request received", "type")

// NewCachedBlobStatter creates a new statter which prefers a cache and
// falls back to a backend.
func NewCachedBlobStatter(cache distribution.BlobDescriptorService, backend distribution.BlobDescriptorService) distribution.BlobDescriptorService {
	return &cachedBlobStatter{
		cache:   cache,
		backend: backend,
	}
}

func (cbds *cachedBlobStatter) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	cacheCount.WithValues("Request").Inc(1)
	desc, err := cbds.cache.Stat(ctx, dgst)
	if err != nil {
		if !errors.Is(err, distribution.ErrBlobUnknown) {
			log.G(ctx).Errorf("error retrieving descriptor from cache: %v", err)
		}

		goto fallback
	}
	cacheCount.WithValues("Hit").Inc(1)
	return desc, nil
fallback:
	cacheCount.WithValues("Miss").Inc(1)
	desc, err = cbds.backend.Stat(ctx, dgst)
	if err != nil {
		return desc, err
	}

	if err := cbds.cache.SetDescriptor(ctx, dgst, desc); err != nil {
		log.G(ctx).Errorf("error adding descriptor %v to cache: %v", desc.Digest, err)
	}

	return desc, err
}

func (cbds *cachedBlobStatter) Clear(ctx context.Context, dgst digest.Digest) error {
	err := cbds.cache.Clear(ctx, dgst)
	if err != nil {
		return err
	}

	err = cbds.backend.Clear(ctx, dgst)
	if err != nil {
		return err
	}
	return nil
}

func (cbds *cachedBlobStatter) SetDescriptor(ctx context.Context, dgst digest.Digest, desc distribution.Descriptor) error {
	if err := cbds.cache.SetDescriptor(ctx, dgst, desc); err != nil {
		log.G(ctx).Errorf("error adding descriptor %v to cache: %v", desc.Digest, err)
	}
	return nil
}
