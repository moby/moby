package layer

import "github.com/docker/distribution"

var _ distribution.Describable = &roLayer{}

func (rl *roLayer) Descriptor() distribution.Descriptor {
	return rl.descriptor
}

func (rl *roLayer) OS() string {
	if rl.os == "" {
		return "windows"
	}
	return rl.os
}
