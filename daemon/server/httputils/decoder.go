package httputils

import (
	"io"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
)

// ContainerDecoder specifies how
// to translate an io.Reader into
// container configuration.
type ContainerDecoder interface {
	DecodeConfig(src io.Reader) (*container.Config, *container.HostConfig, *network.NetworkingConfig, error)
}
