package containerd

import (
	"context"
	"errors"
	"io"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
)

// PushImage initiates a push operation on the repository named localName.
func (i *ImageService) PushImage(ctx context.Context, ref reference.NamedTagged, metaHeaders map[string][]string, authConfig *registry.AuthConfig, outStream io.Writer) (outerr error) {
	return errdefs.NotImplemented(errors.New("not implemented"))
}
