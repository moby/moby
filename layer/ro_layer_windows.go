package layer // import "github.com/docker/docker/layer"

import "github.com/docker/distribution"

var _ distribution.Describable = &roLayer{}

func (rl *roLayer) Descriptor() distribution.Descriptor {
	return rl.descriptor
}
