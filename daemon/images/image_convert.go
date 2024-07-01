package images

import (
	"context"

	"errors"

	"github.com/distribution/reference"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/errdefs"
)

func (i *ImageService) ImageConvert(ctx context.Context, src string, dsts []reference.NamedTagged, opts imagetypes.ConvertOptions) error {
	return errdefs.NotImplemented(errors.New("not supported in graphdriver backed image store"))
}
