package image

import (
	"io"

	"github.com/docker/docker/api/types/backend"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/registry"
	"golang.org/x/net/context"
)

// Backend is all the methods that need to be implemented
// to provide image specific functionality.
type Backend interface {
	containerBackend
	imageBackend
	importExportBackend
	registryBackend
}

type containerBackend interface {
	Commit(name string, config *backend.ContainerCommitConfig) (imageID string, err error)
}

type imageBackend interface {
	ImageDelete(imageRef string, force, prune bool) ([]types.ImageDelete, error)
	ImageHistory(imageName string) ([]*types.ImageHistory, error)
	Images(filterArgs string, filter string, all bool) ([]*types.Image, error)
	LookupImage(name string) (*types.ImageInspect, error)
	TagImage(imageName, repository, tag string) error
}

type importExportBackend interface {
	LoadImage(inTar io.ReadCloser, outStream io.Writer, quiet bool) error
	ImportImage(src string, repository, tag string, msg string, inConfig io.ReadCloser, outStream io.Writer, changes []string) error
	ExportImage(names []string, outStream io.Writer) error
}

type registryBackend interface {
	PullImage(ctx context.Context, image, tag string, metaHeaders map[string][]string, authConfig *types.AuthConfig, outStream io.Writer) error
	PushImage(ctx context.Context, image, tag string, metaHeaders map[string][]string, authConfig *types.AuthConfig, outStream io.Writer) error
	SearchRegistryForImages(ctx context.Context, filtersArgs string, term string, limit int, authConfig *types.AuthConfig, metaHeaders map[string][]string) (*registry.SearchResults, error)
}
