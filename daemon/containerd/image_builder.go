package containerd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/mount"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/stringid"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
)

// GetImageAndReleasableLayer returns an image and releaseable layer for a
// reference or ID. Every call to GetImageAndReleasableLayer MUST call
// releasableLayer.Release() to prevent leaking of layers.
func (i *ImageService) GetImageAndReleasableLayer(ctx context.Context, refOrID string, opts backend.GetImageAndLayerOptions) (builder.Image, builder.ROLayer, error) {
	fmt.Println("GetImageAndReleasableLayer", refOrID)
	img, err := i.GetImage(ctx, refOrID, image.GetImageOpts{})
	if err != nil {
		return nil, nil, err
	}

	c8dImge, err := i.GetContainerdImage(ctx, refOrID, nil)
	if err != nil {
		return nil, nil, err
	}
	ctrdimg := containerd.NewImage(i.client, c8dImge)
	diffIDs, err := ctrdimg.RootFS(ctx)
	if err != nil {
		return nil, nil, err
	}
	parent := identity.ChainID(diffIDs).String()

	s := i.client.SnapshotService(i.snapshotter)
	key := stringid.GenerateRandomID()
	mounts, err := s.View(ctx, key, parent)
	if err != nil {
		return nil, nil, err
	}

	tempMountLocation := os.TempDir()

	root, err := os.MkdirTemp(tempMountLocation, "rootfs-mount")
	if err != nil {
		return nil, nil, err
	}

	if err := mount.All(mounts, root); err != nil {
		return nil, nil, err
	}

	return img, &rolayer{key: key, c: i.client, s: i.snapshotter, diffID: identity.ChainID(diffIDs)}, nil
}

// CreateImage creates a new image by adding a config and ID to the image store.
// This is similar to LoadImage() except that it receives JSON encoded bytes of
// an image instead of a tar archive.
func (i *ImageService) CreateImage(config []byte, parent string) (builder.Image, error) {
	return nil, errdefs.NotImplemented(errors.New("CreateImage not implemented"))
}

type rolayer struct {
	key    string
	c      *containerd.Client
	s      string
	diffID digest.Digest
}

func (rl *rolayer) Release() error {
	// noop...
	return nil
}

func (rl *rolayer) NewRWLayer() (builder.RWLayer, error) {
	s := rl.c.SnapshotService(rl.s)

	key := stringid.GenerateRandomID()
	mounts, err := s.Prepare(context.TODO(), key, rl.key)
	if err != nil {
		return nil, err
	}
	tempMountLocation := os.TempDir()

	root, err := os.MkdirTemp(tempMountLocation, "rootfs-mount")
	if err != nil {
		return nil, err
	}

	if err := mount.All(mounts, root); err != nil {
		return nil, err
	}

	return &rwlayer{s: rl.s, c: rl.c, key: key, parent: rl.key, root: root}, err
}

func (rl *rolayer) DiffID() layer.DiffID {
	return layer.DiffID(rl.diffID)
}

type rwlayer struct {
	key    string
	parent string
	c      *containerd.Client
	s      string
	root   string
}

func (rw *rwlayer) Release() error {
	return nil
}

func (rw *rwlayer) Root() string {
	return rw.root
}

func (rw *rwlayer) Commit() (builder.ROLayer, error) {
	snap := rw.c.SnapshotService(rw.s)
	snap.Commit(context.TODO(), rw.key, rw.parent)

	return nil, nil
}
