//go:build linux || freebsd

package containerd

import (
	"github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/pkg/errors"
)

// GetLayerFolders returns the layer folders from an image RootFS.
func (i *ImageService) GetLayerFolders(img *image.Image, rwLayer container.Layer, containerID string) ([]string, error) {
	return nil, errdefs.NotImplemented(errors.New("not implemented"))
}
