package image

import (
	"io"

	"github.com/docker/docker/reference"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	"github.com/docker/engine-api/types/registry"
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
	Commit(name string, config *types.ContainerCommitConfig) (imageID string, err error)
	Exists(containerName string) bool
}

type imageBackend interface {
	ImageDelete(imageRef string, force, prune bool) ([]types.ImageDelete, error)
	ImageHistory(imageName string) ([]*types.ImageHistory, error)
	Images(filterArgs string, filter string, all bool) ([]*types.Image, error)
	LookupImage(name string) (*types.ImageInspect, error)
	TagImage(newTag reference.Named, imageName string) error
}

type importExportBackend interface {
	LoadImage(inTar io.ReadCloser, outStream io.Writer) error
	ImportImage(src string, newRef reference.Named, msg string, inConfig io.ReadCloser, outStream io.Writer, config *container.Config) error
	ExportImage(names []string, outStream io.Writer) error
}

type registryBackend interface {
	PullImage(ref reference.Named, metaHeaders map[string][]string, authConfig *types.AuthConfig, outStream io.Writer) error
	PushImage(ref reference.Named, metaHeaders map[string][]string, authConfig *types.AuthConfig, outStream io.Writer) error
	SearchRegistryForImages(term string, authConfig *types.AuthConfig, metaHeaders map[string][]string) (*registry.SearchResults, error)
}
