// +build !exclude_volumedriver_localvolumedriver

package volume

import (
	// using stub pattern to load drivers
	_ "github.com/docker/docker/volume/drivers/localvolumedriver"
)
