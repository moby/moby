package image // import "github.com/docker/docker/api/server/router/image"

import (
	"context"
	"io"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	dockerimage "github.com/docker/docker/image"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// Backend is all the methods that need to be implemented
// to provide image specific functionality.
type Backend interface {
	imageBackend
	importExportBackend
	registryBackend
}

type imageBackend interface {
	ImageDelete(ctx context.Context, imageRef string, force, prune bool) ([]types.ImageDeleteResponseItem, error)
	ImageHistory(ctx context.Context, imageName string) ([]*image.HistoryResponseItem, error)
	Images(ctx context.Context, opts types.ImageListOptions) ([]*types.ImageSummary, error)
	GetImage(ctx context.Context, refOrID string, options image.GetImageOpts) (*dockerimage.Image, error)
	TagImage(ctx context.Context, id dockerimage.ID, newRef reference.Named) error
	ImagesPrune(ctx context.Context, pruneFilters filters.Args) (*types.ImagesPruneReport, error)
}

type importExportBackend interface {
	LoadImage(ctx context.Context, inTar io.ReadCloser, outStream io.Writer, quiet bool) error
	ImportImage(ctx context.Context, ref reference.Named, platform *specs.Platform, msg string, layerReader io.Reader, changes []string) (dockerimage.ID, error)
	ExportImage(ctx context.Context, names []string, outStream io.Writer) error
}

type registryBackend interface {
	PullImage(ctx context.Context, image, tag string, platform *specs.Platform, metaHeaders map[string][]string, authConfig *registry.AuthConfig, outStream io.Writer) error
	PushImage(ctx context.Context, image, tag string, metaHeaders map[string][]string, authConfig *registry.AuthConfig, outStream io.Writer) error
}

type Searcher interface {
	Search(ctx context.Context, searchFilters filters.Args, term string, limit int, authConfig *registry.AuthConfig, metaHeaders map[string][]string) ([]registry.SearchResult, error)
}
