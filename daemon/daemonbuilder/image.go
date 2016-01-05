package daemonbuilder

import (
	"github.com/docker/docker/image"
	"github.com/docker/engine-api/types/container"
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
