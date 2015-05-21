// +build !experimental

package daemon

import (
	"fmt"
	"path/filepath"

	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/volume"
	"github.com/docker/docker/volume/drivers"
)

func getVolumeDriver(_ string) (volume.Driver, error) {
	return volumedrivers.Lookup(volume.DefaultDriverName)
}

func parseVolumeSource(spec string, _ *runconfig.Config) (string, string, error) {
	if !filepath.IsAbs(spec) {
		return "", "", fmt.Errorf("cannot bind mount volume: %s volume paths must be absolute.", spec)
	}

	return "", spec, nil
}
