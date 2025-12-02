package containerd

import (
	"context"
	"io"

	"github.com/moby/moby/v2/errdefs"
	"github.com/pkg/errors"
)

// CreateImageDelta generates a binary delta between two images.
// This is currently not implemented for the containerd image store.
func (i *ImageService) CreateImageDelta(ctx context.Context, baseImage, targetImage, tag string, outStream io.Writer) error {
	return errdefs.NotImplemented(errors.New("image delta is not yet supported with containerd image store"))
}
