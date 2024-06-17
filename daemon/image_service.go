package daemon

import (
	"context"
	"io"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	imagetype "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/images"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/archive"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImageService is a temporary interface to assist in the migration to the
// containerd image-store. This interface should not be considered stable,
// and may change over time.
type ImageService interface {
	// Images

	PullImage(ctx context.Context, ref reference.Named, platform *ocispec.Platform, metaHeaders map[string][]string, authConfig *registry.AuthConfig, outStream io.Writer) error
	PushImage(ctx context.Context, ref reference.Named, platform *ocispec.Platform, metaHeaders map[string][]string, authConfig *registry.AuthConfig, outStream io.Writer) error
	CreateImage(ctx context.Context, config []byte, parent string, contentStoreDigest digest.Digest) (builder.Image, error)
	ImageDelete(ctx context.Context, imageRef string, force, prune bool) ([]imagetype.DeleteResponse, error)
	ExportImage(ctx context.Context, names []string, outStream io.Writer) error
	PerformWithBaseFS(ctx context.Context, c *container.Container, fn func(string) error) error
	LoadImage(ctx context.Context, inTar io.ReadCloser, outStream io.Writer, quiet bool) error
	Images(ctx context.Context, opts imagetype.ListOptions) ([]*imagetype.Summary, error)
	LogImageEvent(imageID, refName string, action events.Action)
	CountImages(ctx context.Context) int
	ImagesPrune(ctx context.Context, pruneFilters filters.Args) (*imagetype.PruneReport, error)
	ImportImage(ctx context.Context, ref reference.Named, platform *ocispec.Platform, msg string, layerReader io.Reader, changes []string) (image.ID, error)
	TagImage(ctx context.Context, imageID image.ID, newTag reference.Named) error
	GetImage(ctx context.Context, refOrID string, options backend.GetImageOpts) (*image.Image, error)
	ImageHistory(ctx context.Context, name string) ([]*imagetype.HistoryResponseItem, error)
	CommitImage(ctx context.Context, c backend.CommitConfig) (image.ID, error)
	SquashImage(id, parent string) (string, error)

	// Containerd related methods

	PrepareSnapshot(ctx context.Context, id string, parentImage string, platform *ocispec.Platform, setupInit func(string) error) error
	GetImageManifest(ctx context.Context, refOrID string, options backend.GetImageOpts) (*ocispec.Descriptor, error)

	// Layers

	GetImageAndReleasableLayer(ctx context.Context, refOrID string, opts backend.GetImageAndLayerOptions) (builder.Image, builder.ROLayer, error)
	CreateLayer(container *container.Container, initFunc layer.MountInit) (layer.RWLayer, error)
	LayerStoreStatus() [][2]string
	GetLayerMountID(cid string) (string, error)
	ReleaseLayer(rwlayer layer.RWLayer) error
	LayerDiskUsage(ctx context.Context) (int64, error)
	GetContainerLayerSize(ctx context.Context, containerID string) (int64, int64, error)
	Mount(ctx context.Context, container *container.Container) error
	Unmount(ctx context.Context, container *container.Container) error
	Changes(ctx context.Context, container *container.Container) ([]archive.Change, error)

	// Windows specific

	GetLayerFolders(img *image.Image, rwLayer layer.RWLayer, containerID string) ([]string, error)

	// Build

	MakeImageCache(ctx context.Context, cacheFrom []string) (builder.ImageCache, error)
	CommitBuildStep(ctx context.Context, c backend.CommitConfig) (image.ID, error)

	// Other

	DistributionServices() images.DistributionServices
	Children(ctx context.Context, id image.ID) ([]image.ID, error)
	Cleanup() error
	StorageDriver() string
	UpdateConfig(maxDownloads, maxUploads int)
}
