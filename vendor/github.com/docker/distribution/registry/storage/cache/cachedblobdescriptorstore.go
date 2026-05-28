package cache

import (
	"context"

	"github.com/docker/distribution"
	prometheus "github.com/docker/distribution/metrics"
	"github.com/opencontainers/go-digest"
)

// Metrics is used to hold metric counters
// related to the number of times a cache was
// hit or missed.
type Metrics struct {
	Requests uint64
	Hits     uint64
	Misses   uint64
}

// Logger can be provided on the MetricsTracker to log errors.
//
// Usually, this is just a proxy to dcontext.GetLogger.
type Logger interface {
	Errorf(format string, args ...interface{})
}

// MetricsTracker represents a metric tracker
// which simply counts the number of hits and misses.
type MetricsTracker interface {
	Hit()
	Miss()
	Metrics() Metrics
	Logger(context.Context) Logger
}

type cachedBlobStatter struct {
	cache   distribution.BlobDescriptorService
	backend distribution.BlobDescriptorService
	tracker MetricsTracker
}

var (
	// cacheCount is the number of total cache request received/hits/misses
	cacheCount = prometheus.StorageNamespace.NewLabeledCounter("cache", "The number of cache request received", "type")
)

// NewCachedBlobStatter creates a new statter which prefers a cache and
// falls back to a backend.
func NewCachedBlobStatter(cache distribution.BlobDescriptorService, backend distribution.BlobDescriptorService) distribution.BlobDescriptorService {
	return &cachedBlobStatter{
		cache:   cache,
		backend: backend,
	}
}

// NewCachedBlobStatterWithMetrics creates a new statter which prefers a cache and
// falls back to a backend. Hits and misses will send to the tracker.
func NewCachedBlobStatterWithMetrics(cache distribution.BlobDescriptorService, backend distribution.BlobDescriptorService, tracker MetricsTracker) distribution.BlobStatter {
	return &cachedBlobStatter{
		cache:   cache,
		backend: backend,
		tracker: tracker,
	}
}

func (cbds *cachedBlobStatter) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	cacheCount.WithValues("Request").Inc(1)
	desc, err := cbds.cache.Stat(ctx, dgst)
	if err != nil {
		if err != distribution.ErrBlobUnknown {
			logErrorf(ctx, cbds.tracker, "error retrieving descriptor from cache: %v", err)
		}

		goto fallback
	}
	cacheCount.WithValues("Hit").Inc(1)
	if cbds.tracker != nil {
		cbds.tracker.Hit()
	}
	return desc, nil
fallback:
	cacheCount.WithValues("Miss").Inc(1)
	if cbds.tracker != nil {
		cbds.tracker.Miss()
	}
	desc, err = cbds.backend.Stat(ctx, dgst)
	if err != nil {
		return desc, err
	}

	if err := cbds.cache.SetDescriptor(ctx, dgst, desc); err != nil {
		logErrorf(ctx, cbds.tracker, "error adding descriptor %v to cache: %v", desc.Digest, err)
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
		logErrorf(ctx, cbds.tracker, "error adding descriptor %v to cache: %v", desc.Digest, err)
	}
	return nil
}

func logErrorf(ctx context.Context, tracker MetricsTracker, format string, args ...interface{}) {
	if tracker == nil {
		return
	}

	logger := tracker.Logger(ctx)
	if logger == nil {
		return
	}
	logger.Errorf(format, args...)
}
