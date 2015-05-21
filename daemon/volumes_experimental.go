// +build experimental

package daemon

import (
	"path/filepath"

	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/volume"
	"github.com/docker/docker/volume/drivers"
)

func getVolumeDriver(name string) (volume.Driver, error) {
	if name == "" {
		name = volume.DefaultDriverName
	}
	return volumedrivers.Lookup(name)
}

func parseVolumeSource(spec string, config *runconfig.Config) (string, string, error) {
	if !filepath.IsAbs(spec) {
		return spec, "", nil
	}

	return "", spec, nil
}
