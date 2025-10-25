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
	"github.com/moby/moby/v2/daemon/container"
	daemonevents "github.com/moby/moby/v2/daemon/events"
	dimages "github.com/moby/moby/v2/daemon/images"
	"github.com/moby/moby/v2/daemon/internal/distribution"
	"github.com/moby/moby/v2/daemon/internal/quota"
	"github.com/moby/moby/v2/daemon/snapshotter"
	"github.com/moby/moby/v2/errdefs"
	"github.com/moby/sys/user"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
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
	quotaCtl                *quota.Control
}

type ImageServiceConfig struct {
	Client          *containerd.Client
	Containers      container.Store
	Snapshotter     string
	RootDir         string
	RegistryHosts   docker.RegistryHosts
	Registry        distribution.RegistryResolver
	EventsService   *daemonevents.Events
	RefCountMounter snapshotter.Mounter
	IDMapping       user.IdentityMapping
}

// NewService creates a new ImageService.
func NewService(config ImageServiceConfig) *ImageService {
	var quotaCtl *quota.Control
	if config.RootDir != "" {
		quotaCtl, _ = quota.NewControl(config.RootDir)
	}
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
		quotaCtl:        quotaCtl,
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

	uniqueImages := map[digest.Digest]struct{}{}
	for _, i := range imgs {
		dgst := i.Target().Digest
		if _, ok := uniqueImages[dgst]; !ok {
			uniqueImages[dgst] = struct{}{}
		}
	}

	return len(uniqueImages)
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

// ImageDiskUsage returns the number of bytes used by content and layer stores
// called from disk_usage.go
func (i *ImageService) ImageDiskUsage(ctx context.Context) (int64, error) {
	diskUsage, err := i.layerDiskUsage(ctx)
	if err != nil {
		return 0, err
	}

	// Include the size of content size from the images.
	imgs, err := i.images.List(ctx)
	if err != nil {
		return 0, err
	}

	visitedImages := make(map[digest.Digest]struct{})
	for _, img := range imgs {
		if err := i.walkPresentChildren(ctx, img.Target, func(ctx context.Context, desc ocispec.Descriptor) error {
			if _, ok := visitedImages[desc.Digest]; ok {
				return nil
			}
			visitedImages[desc.Digest] = struct{}{}

			diskUsage += desc.Size
			return nil
		}); err != nil {
			return 0, err
		}
	}
	return diskUsage, nil
}

// LayerDiskUsage returns the number of bytes used by layer stores
// called from disk_usage.go
func (i *ImageService) layerDiskUsage(ctx context.Context) (allLayersSize int64, err error) {
	// TODO(thaJeztah): do we need to take multiple snapshotters into account? See https://github.com/moby/moby/issues/45273
	snapshotter := i.client.SnapshotService(i.snapshotter)
	err = snapshotter.Walk(ctx, func(ctx context.Context, info snapshots.Info) error {
		usage, err := snapshotter.Usage(ctx, info.Name)
		if err != nil {
			return err
		}
		allLayersSize += usage.Size
		return nil
	})
	return allLayersSize, err
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
