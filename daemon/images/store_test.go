package images

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/local"
	c8derrdefs "github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/metadata"
	"github.com/containerd/containerd/namespaces"
	"github.com/docker/docker/image"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"go.etcd.io/bbolt"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func setupTestStores(t *testing.T) (context.Context, content.Store, *imageStoreWithLease, func(t *testing.T)) {
	dir, err := os.MkdirTemp("", t.Name())
	assert.NilError(t, err)

	backend, err := image.NewFSStoreBackend(filepath.Join(dir, "images"))
	assert.NilError(t, err)
	is, err := image.NewImageStore(context.Background(), backend, nil)
	assert.NilError(t, err)

	db, err := bbolt.Open(filepath.Join(dir, "metadata.db"), 0600, nil)
	assert.NilError(t, err)

	cs, err := local.NewStore(filepath.Join(dir, "content"))
	assert.NilError(t, err)
	mdb := metadata.NewDB(db, cs, nil)

	cleanup := func(t *testing.T) {
		assert.Check(t, db.Close())
		assert.Check(t, os.RemoveAll(dir))
	}
	ctx := namespaces.WithNamespace(context.Background(), t.Name())
	images := &imageStoreWithLease{Store: is, leases: metadata.NewLeaseManager(mdb)}

	return ctx, cs, images, cleanup
}

func TestImageDelete(t *testing.T) {
	ctx, _, images, cleanup := setupTestStores(t)
	defer cleanup(t)

	t.Run("no lease", func(t *testing.T) {
		id, err := images.Create(ctx, []byte(`{"rootFS": {}}`))
		assert.NilError(t, err)
		defer images.Delete(ctx, id)

		ls, err := images.leases.List(ctx)
		assert.NilError(t, err)
		assert.Equal(t, len(ls), 0, ls)

		_, err = images.Delete(ctx, id)
		assert.NilError(t, err, "should not error when there is no lease")
	})

	t.Run("lease exists", func(t *testing.T) {
		id, err := images.Create(ctx, []byte(`{"rootFS": {}}`))
		assert.NilError(t, err)
		defer images.Delete(ctx, id)

		leaseID := imageKey(digest.Digest(id))
		_, err = images.leases.Create(ctx, leases.WithID(leaseID))
		assert.NilError(t, err)
		defer images.leases.Delete(ctx, leases.Lease{ID: leaseID})

		ls, err := images.leases.List(ctx)
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(len(ls), 1), ls)

		_, err = images.Delete(ctx, id)
		assert.NilError(t, err)

		ls, err = images.leases.List(ctx)
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(len(ls), 0), ls)
	})
}

func TestContentStoreForPull(t *testing.T) {
	ctx, cs, is, cleanup := setupTestStores(t)
	defer cleanup(t)

	csP := &contentStoreForPull{
		ContentStore: cs,
		leases:       is.leases,
	}

	data := []byte(`{}`)
	desc := v1.Descriptor{
		Digest: digest.Canonical.FromBytes(data),
		Size:   int64(len(data)),
	}

	w, err := csP.Writer(ctx, content.WithRef(t.Name()), content.WithDescriptor(desc))
	assert.NilError(t, err)

	_, err = w.Write(data)
	assert.NilError(t, err)
	defer w.Close()

	err = w.Commit(ctx, desc.Size, desc.Digest)
	assert.NilError(t, err)

	assert.Equal(t, len(csP.digested), 1)
	assert.Check(t, cmp.Equal(csP.digested[0], desc.Digest))

	// Test already exists
	csP.digested = nil
	_, err = csP.Writer(ctx, content.WithRef(t.Name()), content.WithDescriptor(desc))
	assert.Check(t, c8derrdefs.IsAlreadyExists(err))
	assert.Equal(t, len(csP.digested), 1)
	assert.Check(t, cmp.Equal(csP.digested[0], desc.Digest))
}
