// +build experimental

package daemon

import (
	"github.com/docker/docker/volume"
	"github.com/docker/docker/volume/drivers"
)

func getVolumeDriver(name string) (volume.Driver, error) {
	if name == "" {
		name = volume.DefaultDriverName
	}
	return volumedrivers.Lookup(name)
}
