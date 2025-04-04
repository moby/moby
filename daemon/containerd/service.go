package containerd

import (
	"context"
	"fmt"
	"sync/atomic"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/content"
	c8dimages "github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/containerd/v2/plugins"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	"github.com/docker/docker/container"
	daemonevents "github.com/docker/docker/daemon/events"
	dimages "github.com/docker/docker/daemon/images"
	"github.com/docker/docker/daemon/snapshotter"
	"github.com/docker/docker/distribution"
	"github.com/docker/docker/errdefs"
	"github.com/moby/sys/user"
	"github.com/pkg/errors"
)

// ImageService implements daemon.ImageService
type ImageService struct {
	client              *containerd.Client
	images              c8dimages.Store
	content             content.Store
	containers          container.Store
	snapshotterServices map[string]snapshots.Snapshotter
	snapshotter         string
	registryHosts       docker.RegistryHosts
	registryService     distribution.RegistryResolver
	eventsService       *daemonevents.Events
	pruneRunning        atomic.Bool
	refCountMounter     snapshotter.Mounter
	idMapping           user.IdentityMapping

	// defaultPlatformOverride is used in tests to override the host platform.
	defaultPlatformOverride platforms.MatchComparer
}

type ImageServiceConfig struct {
	Client          *containerd.Client
	Containers      container.Store
	Snapshotter     string
	RegistryHosts   docker.RegistryHosts
	Registry        distribution.RegistryResolver
	EventsService   *daemonevents.Events
	RefCountMounter snapshotter.Mounter
	IDMapping       user.IdentityMapping
}

// NewService creates a new ImageService.
func NewService(config ImageServiceConfig) *ImageService {
	return &ImageService{
		client:  config.Client,
		images:  config.Client.ImageService(),
		content: config.Client.ContentStore(),
		snapshotterServices: map[string]snapshots.Snapshotter{
			config.Snapshotter: config.Client.SnapshotService(config.Snapshotter),
		},
		containers:      config.Containers,
		snapshotter:     config.Snapshotter,
		registryHosts:   config.RegistryHosts,
		registryService: config.Registry,
		eventsService:   config.EventsService,
		refCountMounter: config.RefCountMounter,
		idMapping:       config.IDMapping,
	}
}

func (i *ImageService) snapshotterService(snapshotter string) snapshots.Snapshotter {
	s, ok := i.snapshotterServices[snapshotter]
	if !ok {
		s = i.client.SnapshotService(snapshotter)
		i.snapshotterServices[snapshotter] = s
	}

	return s
}

// DistributionServices return services controlling daemon image storage.
func (i *ImageService) DistributionServices() dimages.DistributionServices {
	return dimages.DistributionServices{}
}

// CountImages returns the number of images stored by ImageService
// called from info.go
func (i *ImageService) CountImages(ctx context.Context) int {
	imgs, err := i.client.ListImages(ctx)
	if err != nil {
		return 0
	}

	return len(imgs)
}

// LayerStoreStatus returns the status for each layer store
// called from info.go
func (i *ImageService) LayerStoreStatus() [][2]string {
	// TODO(thaJeztah) do we want to add more details about the driver here?
	return [][2]string{
		{"driver-type", string(plugins.SnapshotPlugin)},
	}
}

// GetLayerMountID returns the mount ID for a layer
// called from daemon.go Daemon.Shutdown(), and Daemon.Cleanup() (cleanup is actually containerCleanup)
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
