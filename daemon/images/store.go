package images

import (
	"context"
	"sync"

	"github.com/containerd/containerd/content"
	c8derrdefs "github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	"github.com/docker/docker/distribution"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func imageKey(dgst digest.Digest) string {
	return "moby-image-" + dgst.String()
}

// imageStoreWithLease wraps the configured image store with one that deletes the lease
// reigstered for a given image ID, if one exists
//
// This is used by the main image service to wrap delete calls to the real image store.
type imageStoreWithLease struct {
	image.Store
	leases leases.Manager

	// Normally we'd pass namespace down through a context.Context, however...
	// The interface for image store doesn't allow this, so we store it here.
	ns string
}

func (s *imageStoreWithLease) Delete(id image.ID) ([]layer.Metadata, error) {
	ctx := namespaces.WithNamespace(context.TODO(), s.ns)
	if err := s.leases.Delete(ctx, leases.Lease{ID: imageKey(digest.Digest(id))}); err != nil && !c8derrdefs.IsNotFound(err) {
		return nil, errors.Wrap(err, "error deleting lease")
	}
	return s.Store.Delete(id)
}

// iamgeStoreForPull is created for each pull It wraps an underlying image store
// to handle registering leases for content fetched in a single image pull.
type imageStoreForPull struct {
	distribution.ImageConfigStore
	leases   leases.Manager
	ingested *contentStoreForPull
}

func (s *imageStoreForPull) Put(ctx context.Context, config []byte) (digest.Digest, error) {
	id, err := s.ImageConfigStore.Put(ctx, config)
	if err != nil {
		return "", err
	}
	return id, s.updateLease(ctx, id)
}

func (s *imageStoreForPull) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	id, err := s.ImageConfigStore.Get(ctx, dgst)
	if err != nil {
		return nil, err
	}
	return id, s.updateLease(ctx, dgst)
}

func (s *imageStoreForPull) updateLease(ctx context.Context, dgst digest.Digest) error {
	leaseID := imageKey(dgst)
	lease, err := s.leases.Create(ctx, leases.WithID(leaseID))
	if err != nil {
		if !c8derrdefs.IsAlreadyExists(err) {
			return errors.Wrap(err, "error creating lease")
		}
		lease = leases.Lease{ID: leaseID}
	}

	digested := s.ingested.getDigested()
	resource := leases.Resource{
		Type: "content",
	}
	for _, dgst := range digested {
		log.G(ctx).WithFields(logrus.Fields{
			"digest": dgst,
			"lease":  lease.ID,
		}).Debug("Adding content digest to lease")

		resource.ID = dgst.String()
		if err := s.leases.AddResource(ctx, lease, resource); err != nil {
			return errors.Wrapf(err, "error adding content digest to lease: %s", dgst)
		}
	}
	return nil
}

// contentStoreForPull is used to wrap the configured content store to
// add lease management for a single `pull`
// It stores all committed digests so that `imageStoreForPull` can add
// the digsted resources to the lease for an image.
type contentStoreForPull struct {
	distribution.ContentStore
	leases leases.Manager

	mu       sync.Mutex
	digested []digest.Digest
}

func (c *contentStoreForPull) addDigested(dgst digest.Digest) {
	c.mu.Lock()
	c.digested = append(c.digested, dgst)
	c.mu.Unlock()
}

func (c *contentStoreForPull) getDigested() []digest.Digest {
	c.mu.Lock()
	digested := make([]digest.Digest, len(c.digested))
	copy(digested, c.digested)
	c.mu.Unlock()
	return digested
}

func (c *contentStoreForPull) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	w, err := c.ContentStore.Writer(ctx, opts...)
	if err != nil {
		if c8derrdefs.IsAlreadyExists(err) {
			var cfg content.WriterOpts
			for _, o := range opts {
				if err := o(&cfg); err != nil {
					return nil, err
				}

			}
			c.addDigested(cfg.Desc.Digest)
		}
		return nil, err
	}
	return &contentWriter{
		cs:     c,
		Writer: w,
	}, nil
}

type contentWriter struct {
	cs *contentStoreForPull
	content.Writer
}

func (w *contentWriter) Commit(ctx context.Context, size int64, expected digest.Digest, opts ...content.Opt) error {
	err := w.Writer.Commit(ctx, size, expected, opts...)
	if err == nil || c8derrdefs.IsAlreadyExists(err) {
		w.cs.addDigested(expected)
	}
	return err
}
