package image

import (
	"io"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/builder/dockerfile"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/runconfig"
)

// Backend is the interface to implement to offer image manipulation capabilities.
type Backend interface {
	Commit(name string, commitCfg *types.ContainerCommitConfig) (string, error)
	PullImage(ref reference.Named, metaHeaders map[string][]string, authConfig *cliconfig.AuthConfig, outStream io.Writer) error
	ImportImage(src string, newRef reference.Named, msg string, inConfig io.ReadCloser, outStream io.Writer, config *runconfig.Config) error
	PushImage(ref reference.Named, metaHeaders map[string][]string, authConfig *cliconfig.AuthConfig, outStream io.Writer) error
	ExportImage(names []string, outStream io.Writer) error
	LoadImage(inTar io.ReadCloser, outStream io.Writer) error
	ImageDelete(imageRef string, force, prune bool) ([]types.ImageDelete, error)
	LookupImage(name string) (*types.ImageInspect, error)
	Images(filterArgs, filter string, all bool) ([]*types.Image, error)
	ImageHistory(name string) ([]*types.ImageHistory, error)
	TagImage(newTag reference.Named, imageName string) error
	SearchRegistryForImages(term string, authConfig *cliconfig.AuthConfig, headers map[string][]string) (*registry.SearchResults, error)
	ImageBuild(buildConfig *dockerfile.Config, input io.ReadCloser, output io.Writer) error
}
