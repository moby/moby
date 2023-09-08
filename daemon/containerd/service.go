package containerd

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/containerd/containerd"
	cerrdefs "github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/snapshots"
	"github.com/distribution/reference"
	"github.com/docker/docker/container"
	daemonevents "github.com/docker/docker/daemon/events"
	"github.com/docker/docker/daemon/images"
	"github.com/docker/docker/daemon/snapshotter"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/registry"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// ImageService implements daemon.ImageService
type ImageService struct {
	client          *containerd.Client
	containers      container.Store
	snapshotter     string
	registryHosts   docker.RegistryHosts
	registryService RegistryConfigProvider
	eventsService   *daemonevents.Events
	pruneRunning    atomic.Bool
	refCountMounter snapshotter.Mounter
}

type RegistryConfigProvider interface {
	IsInsecureRegistry(host string) bool
	ResolveRepository(name reference.Named) (*registry.RepositoryInfo, error)
}

type ImageServiceConfig struct {
	Client          *containerd.Client
	Containers      container.Store
	Snapshotter     string
	RegistryHosts   docker.RegistryHosts
	Registry        RegistryConfigProvider
	EventsService   *daemonevents.Events
	RefCountMounter snapshotter.Mounter
}

// NewService creates a new ImageService.
func NewService(config ImageServiceConfig) *ImageService {
	return &ImageService{
		client:          config.Client,
		containers:      config.Containers,
		snapshotter:     config.Snapshotter,
		registryHosts:   config.RegistryHosts,
		registryService: config.Registry,
		eventsService:   config.EventsService,
		refCountMounter: config.RefCountMounter,
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

// CreateLayer creates a filesystem layer for a container.
// called from create.go
// TODO: accept an opt struct instead of container?
func (i *ImageService) CreateLayer(container *container.Container, initFunc layer.MountInit) (layer.RWLayer, error) {
	return nil, errdefs.NotImplemented(errdefs.NotImplemented(errors.New("not implemented")))
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
	// TODO(thaJeztah): do we need to take multiple snapshotters into account? See https://github.com/moby/moby/issues/45273
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
	log.G(context.TODO()).Warn("max downloads and uploads is not yet implemented with the containerd store")
}

// GetLayerFolders returns the layer folders from an image RootFS.
func (i *ImageService) GetLayerFolders(img *image.Image, rwLayer layer.RWLayer) ([]string, error) {
	return nil, errdefs.NotImplemented(errors.New("not implemented"))
}

// GetContainerLayerSize returns the real size & virtual size of the container.
func (i *ImageService) GetContainerLayerSize(ctx context.Context, containerID string) (int64, int64, error) {
	ctr := i.containers.Get(containerID)
	if ctr == nil {
		return 0, 0, nil
	}

	snapshotter := i.client.SnapshotService(ctr.Driver)
	rwLayerUsage, err := snapshotter.Usage(ctx, containerID)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return 0, 0, errdefs.NotFound(fmt.Errorf("rw layer snapshot not found for container %s", containerID))
		}
		return 0, 0, errdefs.System(errors.Wrapf(err, "snapshotter.Usage failed for %s", containerID))
	}

	unpackedUsage, err := calculateSnapshotParentUsage(ctx, snapshotter, containerID)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			log.G(ctx).WithField("ctr", containerID).Warn("parent of container snapshot no longer present")
		} else {
			log.G(ctx).WithError(err).WithField("ctr", containerID).Warn("unexpected error when calculating usage of the parent snapshots")
		}
	}
	log.G(ctx).WithFields(log.Fields{
		"rwLayerUsage": rwLayerUsage.Size,
		"unpacked":     unpackedUsage.Size,
	}).Debug("GetContainerLayerSize")

	// TODO(thaJeztah): include content-store size for the image (similar to "GET /images/json")
	return rwLayerUsage.Size, rwLayerUsage.Size + unpackedUsage.Size, nil
}

// getContainerImageManifest safely dereferences ImageManifest.
// ImageManifest can be nil for containers created with Docker Desktop with old
// containerd image store integration enabled which didn't set this field.
func getContainerImageManifest(ctr *container.Container) (ocispec.Descriptor, error) {
	if ctr.ImageManifest == nil {
		return ocispec.Descriptor{}, errdefs.InvalidParameter(errors.New("container is missing ImageManifest (probably created on old version), please recreate it"))
	}

	return *ctr.ImageManifest, nil
}
