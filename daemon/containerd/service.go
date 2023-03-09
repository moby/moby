package containerd

import (
	"context"
	"encoding/json"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/snapshots"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/images"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/registry"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// ImageService implements daemon.ImageService
type ImageService struct {
	client          *containerd.Client
	containers      container.Store
	snapshotter     string
	registryHosts   RegistryHostsProvider
	registryService registry.Service
}

type RegistryHostsProvider interface {
	RegistryHosts() docker.RegistryHosts
}

// NewService creates a new ImageService.
func NewService(c *containerd.Client, containers container.Store, snapshotter string, hostsProvider RegistryHostsProvider, registry registry.Service) *ImageService {
	return &ImageService{
		client:          c,
		containers:      containers,
		snapshotter:     snapshotter,
		registryHosts:   hostsProvider,
		registryService: registry,
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
	var allLayersSize int64
	snapshotter := i.client.SnapshotService(i.snapshotter)
	snapshotter.Walk(ctx, func(ctx context.Context, info snapshots.Info) error {
		usage, err := snapshotter.Usage(ctx, info.Name)
		if err != nil {
			return err
		}
		allLayersSize += usage.Size
		return nil
	})
	return allLayersSize, nil
}

// UpdateConfig values
//
// called from reload.go
func (i *ImageService) UpdateConfig(maxDownloads, maxUploads int) {
	panic("not implemented")
}

// GetContainerLayerSize returns the real size & virtual size of the container.
func (i *ImageService) GetContainerLayerSize(ctx context.Context, containerID string) (int64, int64, error) {
	ctr := i.containers.Get(containerID)
	if ctr == nil {
		return 0, 0, nil
	}
	cs := i.client.ContentStore()

	imageManifestBytes, err := content.ReadBlob(ctx, cs, *ctr.ImageManifest)
	if err != nil {
		return 0, 0, err
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(imageManifestBytes, &manifest); err != nil {
		return 0, 0, err
	}

	imageConfigBytes, err := content.ReadBlob(ctx, cs, manifest.Config)
	if err != nil {
		return 0, 0, err
	}
	var img ocispec.Image
	if err := json.Unmarshal(imageConfigBytes, &img); err != nil {
		return 0, 0, err
	}

	snapshotter := i.client.SnapshotService(i.snapshotter)
	usage, err := snapshotter.Usage(ctx, containerID)
	if err != nil {
		return 0, 0, err
	}

	sizeCache := make(map[digest.Digest]int64)
	snapshotSizeFn := func(d digest.Digest) (int64, error) {
		if s, ok := sizeCache[d]; ok {
			return s, nil
		}
		u, err := snapshotter.Usage(ctx, d.String())
		if err != nil {
			return 0, err
		}
		sizeCache[d] = u.Size
		return u.Size, nil
	}

	chainIDs := identity.ChainIDs(img.RootFS.DiffIDs)
	virtualSize, err := computeVirtualSize(chainIDs, snapshotSizeFn)
	if err != nil {
		return 0, 0, err
	}

	return usage.Size, usage.Size + virtualSize, nil
}
