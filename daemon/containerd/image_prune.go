package containerd

import (
	"context"
	"errors"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/errdefs"
)

// ImagesPrune removes unused images
func (i *ImageService) ImagesPrune(ctx context.Context, pruneFilters filters.Args) (*types.ImagesPruneReport, error) {
	return nil, errdefs.NotImplemented(errors.New("not implemented"))
}
