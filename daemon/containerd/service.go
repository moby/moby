package containerd

import (
	"context"
	"encoding/json"
	"sync/atomic"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/snapshots"
	"github.com/docker/distribution/reference"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/container"
	daemonevents "github.com/docker/docker/daemon/events"
	"github.com/docker/docker/daemon/images"
	"github.com/docker/docker/daemon/snapshotter"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/registry"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ImageService implements daemon.ImageService
type ImageService struct {
	client          *containerd.Client
	containers      container.Store
	snapshotter     string
	registryHosts   RegistryHostsProvider
	registryService RegistryConfigProvider
	eventsService   *daemonevents.Events
	pruneRunning    atomic.Bool
	refCountMounter snapshotter.Mounter
}

type RegistryHostsProvider interface {
	RegistryHosts() docker.RegistryHosts
}

type RegistryConfigProvider interface {
	IsInsecureRegistry(host string) bool
	ResolveRepository(name reference.Named) (*registry.RepositoryInfo, error)
}

type ImageServiceConfig struct {
	Client          *containerd.Client
	Containers      container.Store
	Snapshotter     string
	HostsProvider   RegistryHostsProvider
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
		registryHosts:   config.HostsProvider,
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
	panic("not implemented")
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

	usage, err := snapshotter.Usage(ctx, containerID)
	if err != nil {
		return 0, 0, err
	}

	imageManifest, err := getContainerImageManifest(ctr)
	if err != nil {
		// Best efforts attempt to pick an image.
		// We don't have platform information at this point, so we can only
		// assume that the platform matches host.
		// Otherwise this will give a wrong base image size (different
		// platform), but should be close enough.
		mfst, err := i.GetImageManifest(ctx, ctr.Config.Image, imagetypes.GetImageOpts{})
		if err != nil {
			// Log error, don't error out whole operation.
			logrus.WithFields(logrus.Fields{
				logrus.ErrorKey: err,
				"container":     containerID,
			}).Warn("empty ImageManifest, can't calculate base image size")
			return usage.Size, 0, nil
		}
		imageManifest = *mfst
	}
	cs := i.client.ContentStore()

	imageManifestBytes, err := content.ReadBlob(ctx, cs, imageManifest)
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
	snapShotSize, err := computeSnapshotSize(chainIDs, snapshotSizeFn)
	if err != nil {
		return 0, 0, err
	}

	// TODO(thaJeztah): include content-store size for the image (similar to "GET /images/json")
	return usage.Size, usage.Size + snapShotSize, nil
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
