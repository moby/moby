package daemonbuilder

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/image"
)

type imgWrap struct {
	inner *image.Image
}

func (img imgWrap) ID() string {
	return string(img.inner.ID())
}

func (img imgWrap) Config() *container.Config {
	return img.inner.Config
}
