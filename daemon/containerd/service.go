package containerd

import (
	"context"
	"errors"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/plugin"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/images"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
)

// ImageService implements daemon.ImageService
type ImageService struct {
	client      *containerd.Client
	snapshotter string
}

// NewService creates a new ImageService.
func NewService(c *containerd.Client, snapshotter string) *ImageService {
	return &ImageService{
		client:      c,
		snapshotter: snapshotter,
	}
}

// DistributionServices return services controlling daemon image storage.
func (i *ImageService) DistributionServices() images.DistributionServices {
	return images.DistributionServices{}
}

// CountImages returns the number of images stored by ImageService
// called from info.go
func (i *ImageService) CountImages() int {
	imgs, err := i.client.ListImages(context.TODO())
	if err != nil {
		return 0
	}

	return len(imgs)
}

// Children returns the children image.IDs for a parent image.
// called from list.go to filter containers
// TODO: refactor to expose an ancestry for image.ID?
func (i *ImageService) Children(id image.ID) []image.ID {
	panic("not implemented")
}

// CreateLayer creates a filesystem layer for a container.
// called from create.go
// TODO: accept an opt struct instead of container?
func (i *ImageService) CreateLayer(container *container.Container, initFunc layer.MountInit) (layer.RWLayer, error) {
	return nil, errdefs.NotImplemented(errdefs.NotImplemented(errors.New("not implemented")))
}

// GetLayerByID returns a layer by ID
// called from daemon.go Daemon.restore(), and Daemon.containerExport().
func (i *ImageService) GetLayerByID(cid string) (layer.RWLayer, error) {
	return nil, errdefs.NotImplemented(errors.New("not implemented"))
}

// LayerStoreStatus returns the status for each layer store
// called from info.go
func (i *ImageService) LayerStoreStatus() [][2]string {
	// TODO(thaJeztah) do we want to add more details about the driver here?
	return [][2]string{
		{"driver-type", string(plugin.SnapshotPlugin)},
	}
}

// GetLayerMountID returns the mount ID for a layer
// called from daemon.go Daemon.Shutdown(), and Daemon.Cleanup() (cleanup is actually continerCleanup)
// TODO: needs to be refactored to Unmount (see callers), or removed and replaced with GetLayerByID
func (i *ImageService) GetLayerMountID(cid string) (string, error) {
	return "", errdefs.NotImplemented(errors.New("not implemented"))
}

// Cleanup resources before the process is shutdown.
// called from daemon.go Daemon.Shutdown()
func (i *ImageService) Cleanup() error {
	return nil
}

// StorageDriver returns the name of the default storage-driver (snapshotter)
// used by the ImageService.
func (i *ImageService) StorageDriver() string {
	return i.snapshotter
}

// ReleaseLayer releases a layer allowing it to be removed
// called from delete.go Daemon.cleanupContainer(), and Daemon.containerExport()
func (i *ImageService) ReleaseLayer(rwlayer layer.RWLayer) error {
	return errdefs.NotImplemented(errors.New("not implemented"))
}

// LayerDiskUsage returns the number of bytes used by layer stores
// called from disk_usage.go
func (i *ImageService) LayerDiskUsage(ctx context.Context) (int64, error) {
	return 0, errdefs.NotImplemented(errors.New("not implemented"))
}

// ImageDiskUsage returns information about image data disk usage.
func (i *ImageService) ImageDiskUsage(ctx context.Context) ([]*types.ImageSummary, error) {
	return nil, errdefs.NotImplemented(errors.New("not implemented"))
}

// UpdateConfig values
//
// called from reload.go
func (i *ImageService) UpdateConfig(maxDownloads, maxUploads int) {
	panic("not implemented")
}

// GetLayerFolders returns the layer folders from an image RootFS.
func (i *ImageService) GetLayerFolders(img *image.Image, rwLayer layer.RWLayer) ([]string, error) {
	return nil, errdefs.NotImplemented(errors.New("not implemented"))
}

// GetContainerLayerSize returns the real size & virtual size of the container.
func (i *ImageService) GetContainerLayerSize(ctx context.Context, containerID string) (int64, int64, error) {
	snapshotter := i.client.SnapshotService(containerd.DefaultSnapshotter)
	sizeCache := make(map[digest.Digest]int64)
	snapshotSizeFn := func(d digest.Digest) (int64, error) {
		if s, ok := sizeCache[d]; ok {
			return s, nil
		}
		usage, err := snapshotter.Usage(ctx, d.String())
		if err != nil {
			return 0, err
		}
		sizeCache[d] = usage.Size
		return usage.Size, nil
	}

	c, err := i.client.ContainerService().Get(ctx, containerID)
	if err != nil {
		return 0, 0, err
	}
	image, err := i.client.GetImage(ctx, c.Image)
	if err != nil {
		return 0, 0, err
	}
	diffIDs, err := image.RootFS(ctx)
	if err != nil {
		return 0, 0, err
	}
	chainIDs := identity.ChainIDs(diffIDs)

	usage, err := snapshotter.Usage(ctx, containerID)
	if err != nil {
		return 0, 0, err
	}
	size := usage.Size

	virtualSize, err := computeVirtualSize(chainIDs, snapshotSizeFn)
	if err != nil {
		return 0, 0, err
	}
	return size, size + virtualSize, nil
}
