package daemon

import (
	"context"
	"io"

	"github.com/docker/distribution"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/filters"
	imagetype "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/images"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImageService is a temporary interface to assist in the migration to the
// containerd image-store. This interface should not be considered stable,
// and may change over time.
type ImageService interface {
	// Images

	PullImage(ctx context.Context, image, tag string, platform *v1.Platform, metaHeaders map[string][]string, authConfig *types.AuthConfig, outStream io.Writer) error
	PushImage(ctx context.Context, image, tag string, metaHeaders map[string][]string, authConfig *types.AuthConfig, outStream io.Writer) error
	CreateImage(config []byte, parent string) (builder.Image, error)
	ImageDelete(imageRef string, force, prune bool) ([]types.ImageDeleteResponseItem, error)
	ExportImage(names []string, outStream io.Writer) error
	LoadImage(inTar io.ReadCloser, outStream io.Writer, quiet bool) error
	Images(ctx context.Context, opts types.ImageListOptions) ([]*types.ImageSummary, error)
	LogImageEvent(imageID, refName, action string)
	LogImageEventWithAttributes(imageID, refName, action string, attributes map[string]string)
	CountImages() int
	ImageDiskUsage(ctx context.Context) ([]*types.ImageSummary, error)
	ImagesPrune(ctx context.Context, pruneFilters filters.Args) (*types.ImagesPruneReport, error)
	ImportImage(ctx context.Context, src string, repository string, platform *v1.Platform, tag string, msg string, inConfig io.ReadCloser, outStream io.Writer, changes []string) error
	TagImage(imageName, repository, tag string) (string, error)
	TagImageWithReference(imageID image.ID, newTag reference.Named) error
	GetImage(ctx context.Context, refOrID string, platform *v1.Platform) (*image.Image, error)
	ImageHistory(name string) ([]*imagetype.HistoryResponseItem, error)
	CommitImage(c backend.CommitConfig) (image.ID, error)
	SquashImage(id, parent string) (string, error)

	// Layers

	GetImageAndReleasableLayer(ctx context.Context, refOrID string, opts backend.GetImageAndLayerOptions) (builder.Image, builder.ROLayer, error)
	CreateLayer(container *container.Container, initFunc layer.MountInit) (layer.RWLayer, error)
	GetLayerByID(cid string) (layer.RWLayer, error)
	LayerStoreStatus() [][2]string
	GetLayerMountID(cid string) (string, error)
	ReleaseLayer(rwlayer layer.RWLayer) error
	LayerDiskUsage(ctx context.Context) (int64, error)
	GetContainerLayerSize(containerID string) (int64, int64)

	// Windows specific

	GetLayerFolders(img *image.Image, rwLayer layer.RWLayer) ([]string, error)

	// Build

	MakeImageCache(ctx context.Context, cacheFrom []string) builder.ImageCache
	CommitBuildStep(c backend.CommitConfig) (image.ID, error)

	// Other

	GetRepository(ctx context.Context, ref reference.Named, authConfig *types.AuthConfig) (distribution.Repository, error)
	Map() map[image.ID]*image.Image
	SearchRegistryForImages(ctx context.Context, searchFilters filters.Args, term string, limit int, authConfig *types.AuthConfig, headers map[string][]string) (*registry.SearchResults, error)
	DistributionServices() images.DistributionServices
	Children(id image.ID) []image.ID
	Cleanup() error
	GraphDriverName() string
	UpdateConfig(maxDownloads, maxUploads int)
}
