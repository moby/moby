package daemon

import (
	"github.com/docker/docker/image"
	"github.com/docker/docker/runconfig"
)

// createContainerPlatformSpecificSettings performs platform specific container create functionality
func createContainerPlatformSpecificSettings(container *Container, config *runconfig.Config, img *image.Image) error {
	return nil
}
