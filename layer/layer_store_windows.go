package layer

import (
	"io"

	"github.com/docker/distribution"
)

func (ls *layerStore) RegisterWithDescriptor(ts io.Reader, parent ChainID, os string, descriptor distribution.Descriptor) (Layer, error) {
	return ls.registerWithDescriptor(ts, parent, os, descriptor)
}
