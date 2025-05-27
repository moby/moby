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
	"github.com/moby/go-archive"
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
	ImageDelete(ctx context.Context, imageRef string, options imagetype.RemoveOptions) ([]imagetype.DeleteResponse, error)
	ExportImage(ctx context.Context, names []string, platform *ocispec.Platform, outStream io.Writer) error
	LoadImage(ctx context.Context, inTar io.ReadCloser, platform *ocispec.Platform, outStream io.Writer, quiet bool) error
	Images(ctx context.Context, opts imagetype.ListOptions) ([]*imagetype.Summary, error)
	LogImageEvent(ctx context.Context, imageID, refName string, action events.Action)
	CountImages(ctx context.Context) int
	ImagesPrune(ctx context.Context, pruneFilters filters.Args) (*imagetype.PruneReport, error)
	ImportImage(ctx context.Context, ref reference.Named, platform *ocispec.Platform, msg string, layerReader io.Reader, changes []string) (image.ID, error)
	TagImage(ctx context.Context, imageID image.ID, newTag reference.Named) error
	GetImage(ctx context.Context, refOrID string, options backend.GetImageOpts) (*image.Image, error)
	ImageHistory(ctx context.Context, name string, platform *ocispec.Platform) ([]*imagetype.HistoryResponseItem, error)
	CommitImage(ctx context.Context, c backend.CommitConfig) (image.ID, error)
	SquashImage(id, parent string) (string, error)
	ImageInspect(ctx context.Context, refOrID string, opts backend.ImageInspectOpts) (*imagetype.InspectResponse, error)
	ImageDiskUsage(ctx context.Context) (int64, error)

	// Layers

	GetImageAndReleasableLayer(ctx context.Context, refOrID string, opts backend.GetImageAndLayerOptions) (builder.Image, builder.ROLayer, error)
	CreateLayer(container *container.Container, initFunc layer.MountInit) (container.RWLayer, error)
	CreateLayerFromImage(img *image.Image, layerName string, rwLayerOpts *layer.CreateRWLayerOpts) (container.RWLayer, error)
	GetLayerByID(cid string) (container.RWLayer, error)
	LayerStoreStatus() [][2]string
	GetLayerMountID(cid string) (string, error)
	ReleaseLayer(rwlayer container.RWLayer) error
	GetContainerLayerSize(ctx context.Context, containerID string) (int64, int64, error)
	Changes(ctx context.Context, container *container.Container) ([]archive.Change, error)

	// Windows specific

	GetLayerFolders(img *image.Image, rwLayer container.RWLayer, containerID string) ([]string, error)

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
