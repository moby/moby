// +build !experimental

package daemon

import (
	"github.com/docker/docker/volume"
	"github.com/docker/docker/volume/drivers"
)

func getVolumeDriver(_ string) (volume.Driver, error) {
	return volumedrivers.Lookup(volume.DefaultDriverName)
}
