package containerd

import (
	"context"
	"errors"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
)

// SearchRegistryForImages queries the registry for images matching
// term. authConfig is used to login.
//
// TODO: this could be implemented in a registry service instead of the image
// service.
func (i *ImageService) SearchRegistryForImages(ctx context.Context, searchFilters filters.Args, term string, limit int, authConfig *registry.AuthConfig, metaHeaders map[string][]string) (*registry.SearchResults, error) {
	return nil, errdefs.NotImplemented(errors.New("SearchRegistryForImages not implemented"))
}
