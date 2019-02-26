package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"os"
	"runtime"
	"sync"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/docker/container"
	daemonevents "github.com/docker/docker/daemon/events"
	"github.com/docker/docker/distribution"
	"github.com/docker/docker/distribution/metadata"
	"github.com/docker/docker/distribution/xfer"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	dockerreference "github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type containerStore interface {
	// used by image delete
	First(container.StoreFilter) *container.Container
	// used by image prune, and image list
	List() []*container.Container
	// TODO: remove, only used for CommitBuildStep
	Get(string) *container.Container
}

// ImageServiceConfig is the configuration used to create a new ImageService
type ImageServiceConfig struct {
	DefaultNamespace          string
	Client                    *containerd.Client
	ContainerStore            containerStore
	DistributionMetadataStore metadata.Store
	EventsService             *daemonevents.Events
	ImageStore                image.Store
	LayerStores               map[string]layer.Store
	MaxConcurrentDownloads    int
	MaxConcurrentUploads      int
	ReferenceStore            dockerreference.Store
	RegistryService           registry.Service
}

// NewImageService returns a new ImageService from a configuration
func NewImageService(config ImageServiceConfig) *ImageService {
	logrus.Debugf("Max Concurrent Downloads: %d", config.MaxConcurrentDownloads)
	logrus.Debugf("Max Concurrent Uploads: %d", config.MaxConcurrentUploads)
	return &ImageService{
		namespace:                 config.DefaultNamespace,
		client:                    config.Client,
		containers:                config.ContainerStore,
		distributionMetadataStore: config.DistributionMetadataStore,
		downloadManager:           xfer.NewLayerDownloadManager(config.LayerStores, config.MaxConcurrentDownloads),
		cache:                     map[string]*cache{},
		eventsService:             config.EventsService,
		imageStore:                config.ImageStore,
		layerStores:               config.LayerStores,
		referenceStore:            config.ReferenceStore,
		registryService:           config.RegistryService,
		uploadManager:             xfer.NewLayerUploadManager(config.MaxConcurrentUploads),

		// TODO(containerd): derive from configured layerstores
		platforms: platforms.Ordered(platforms.DefaultSpec()),
	}
}

// ImageService provides a backend for image management
type ImageService struct {
	namespace     string
	client        *containerd.Client
	containers    containerStore
	eventsService *daemonevents.Events
	layerStores   map[string]layer.Store // By operating system
	platforms     platforms.MatchComparer
	pruneRunning  int32

	// namespaced cache
	cache  map[string]*cache
	cacheL sync.RWMutex

	// To be replaced by containerd client
	registryService           registry.Service
	referenceStore            dockerreference.Store
	imageStore                image.Store
	distributionMetadataStore metadata.Store
	downloadManager           *xfer.LayerDownloadManager
	uploadManager             *xfer.LayerUploadManager
}

// DistributionServices provides daemon image storage services
type DistributionServices struct {
	DownloadManager   distribution.RootFSDownloadManager
	V2MetadataService metadata.V2MetadataService
	LayerStore        layer.Store // TODO: lcow
	ImageStore        image.Store
	ReferenceStore    dockerreference.Store
}

// DistributionServices return services controlling daemon image storage
func (i *ImageService) DistributionServices() DistributionServices {
	return DistributionServices{
		DownloadManager:   i.downloadManager,
		V2MetadataService: metadata.NewV2MetadataService(i.distributionMetadataStore),
		LayerStore:        i.layerStores[runtime.GOOS],
		ImageStore:        i.imageStore,
		ReferenceStore:    i.referenceStore,
	}
}

// CountImages returns the number of images stored by ImageService
// called from info.go
func (i *ImageService) CountImages(ctx context.Context) (int, error) {
	c, err := i.getCache(ctx)
	if err != nil {
		return 0, err
	}

	c.m.RLock()
	l := len(c.idCache)
	c.m.RUnlock()

	return l, nil
}

// ChildrenByID returns the children image digests for a parent image.
// called from list.go to filter containers
func (i *ImageService) ChildrenByID(ctx context.Context, id digest.Digest) ([]digest.Digest, error) {
	c, err := i.getCache(ctx)
	if err != nil {
		return nil, err
	}

	c.m.RLock()
	ci, ok := c.idCache[id]
	c.m.RUnlock()
	if !ok {
		return nil, nil
	}

	return ci.children, nil
}

// GetImageBackend returns the storage backend used by the given image
// TODO(containerd): return more abstract interface to support snapshotters
func (i *ImageService) GetImageBackend(image RuntimeImage) (layer.Store, error) {
	if image.Config.Digest != "" {
		// TODO(containerd): Get from content-store label
		// TODO(containerd): Lookup by layer store names
	}
	if image.Platform.OS == "" {
		image.Platform = platforms.DefaultSpec()
	}
	ls, ok := i.layerStores[image.Platform.OS]
	if !ok {
		return nil, errdefs.Unavailable(errors.Errorf("no storage backend configured for %s", image.Platform.OS))
	}

	return ls, nil
}

// GetLayerByID returns a layer by ID and operating system
// called from daemon.go Daemon.restore(), and Daemon.containerExport()
func (i *ImageService) GetLayerByID(cid string, os string) (layer.RWLayer, error) {
	return i.layerStores[os].GetRWLayer(cid)
}

// LayerStoreStatus returns the status for each layer store
// called from info.go
func (i *ImageService) LayerStoreStatus() map[string][][2]string {
	result := make(map[string][][2]string)
	for os, store := range i.layerStores {
		result[os] = store.DriverStatus()
	}
	return result
}

// GetLayerMountID returns the mount ID for a layer
// called from daemon.go Daemon.Shutdown(), and Daemon.Cleanup() (cleanup is actually continerCleanup)
// TODO: needs to be refactored to Unmount (see callers), or removed and replaced
// with GetLayerByID
func (i *ImageService) GetLayerMountID(cid string, os string) (string, error) {
	return i.layerStores[os].GetMountID(cid)
}

// Cleanup resources before the process is shutdown.
// called from daemon.go Daemon.Shutdown()
func (i *ImageService) Cleanup() {
	for os, ls := range i.layerStores {
		if ls != nil {
			if err := ls.Cleanup(); err != nil {
				logrus.Errorf("Error during layer Store.Cleanup(): %v %s", err, os)
			}
		}
	}
}

// GraphDriverForOS returns the name of the graph drvier
// moved from Daemon.GraphDriverName, used by:
// - newContainer
// - to report an error in Daemon.Mount(container)
func (i *ImageService) GraphDriverForOS(os string) string {
	return i.layerStores[os].DriverName()
}

// ReleaseLayer releases a layer allowing it to be removed
// called from delete.go Daemon.cleanupContainer(), and Daemon.containerExport()
func (i *ImageService) ReleaseLayer(rwlayer layer.RWLayer, containerOS string) error {
	metadata, err := i.layerStores[containerOS].ReleaseRWLayer(rwlayer)
	layer.LogReleaseMetadata(metadata)
	if err != nil && err != layer.ErrMountDoesNotExist && !os.IsNotExist(errors.Cause(err)) {
		return errors.Wrapf(err, "driver %q failed to remove root filesystem",
			i.layerStores[containerOS].DriverName())
	}
	return nil
}

// LayerDiskUsage returns the number of bytes used by layer stores
// called from disk_usage.go
func (i *ImageService) LayerDiskUsage(ctx context.Context) (int64, error) {
	var allLayersSize int64
	layerRefs, err := i.getLayerRefs(ctx)
	if err != nil {
		return 0, err
	}
	for _, ls := range i.layerStores {
		allLayers := ls.Map()
		for _, l := range allLayers {
			select {
			case <-ctx.Done():
				return allLayersSize, ctx.Err()
			default:
				size, err := l.DiffSize()
				if err == nil {
					if _, ok := layerRefs[digest.Digest(l.ChainID())]; ok {
						allLayersSize += size
					}
				} else {
					logrus.Warnf("failed to get diff size for layer %v", l.ChainID())
				}
			}
		}
	}
	return allLayersSize, nil
}

func (i *ImageService) getLayerRefs(ctx context.Context) (map[digest.Digest]int, error) {
	c, err := i.getCache(ctx)
	if err != nil {
		return nil, err
	}

	// Create copy and unlock cache
	c.m.RLock()
	imgs := make(map[digest.Digest]*cachedImage, len(c.idCache))
	for dgst, ci := range c.idCache {
		imgs[dgst] = ci
	}
	c.m.RUnlock()

	layerRefs := map[digest.Digest]int{}
	for _, img := range imgs {
		if len(img.references) == 0 && len(img.children) != 0 {
			continue
		}

		diffIDs, err := images.RootFS(ctx, i.client.ContentStore(), img.config)
		if err != nil {
			if errdefs.IsNotFound(err) {
				continue
			}
			return nil, errors.Wrap(err, "failed to resolve rootfs")
		}

		for i := range diffIDs {
			layerRefs[identity.ChainID(diffIDs[:i+1])]++
		}
	}

	return layerRefs, nil
}

// UpdateConfig values
//
// called from reload.go
func (i *ImageService) UpdateConfig(maxDownloads, maxUploads *int) {
	// TODO(containerd): store these locally to configure resolver
	if i.downloadManager != nil && maxDownloads != nil {
		i.downloadManager.SetConcurrency(*maxDownloads)
	}
	if i.uploadManager != nil && maxUploads != nil {
		i.uploadManager.SetConcurrency(*maxUploads)
	}
}
