package containerd

import (
	"context"
	"io"

	"github.com/docker/docker/api/types/registry"
)

// PushImage initiates a push operation on the repository named localName.
func (i *ImageService) PushImage(ctx context.Context, image, tag string, metaHeaders map[string][]string, authConfig *registry.AuthConfig, outStream io.Writer) error {
	panic("not implemented")
}
