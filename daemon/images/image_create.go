package images

import (
	"context"
	"io"

	"errors"

	"github.com/distribution/reference"
	"github.com/docker/docker/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func (i *ImageService) ImageCreateFromJSON(ctx context.Context, ref reference.NamedTagged, jsonReader io.Reader) (ocispec.Descriptor, error) {
	return ocispec.Descriptor{}, errdefs.NotImplemented(errors.New("not supported in graphdriver backed image store"))
}
