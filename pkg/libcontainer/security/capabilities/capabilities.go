package capabilities

import (
	"os"

	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/syndtr/gocapability/capability"
)

// DropCapabilities drops capabilities for the current process based
// on the container's configuration.
func DropCapabilities(container *libcontainer.Container) error {
	if drop := getCapabilitiesMask(container); len(drop) > 0 {
		c, err := capability.NewPid(os.Getpid())
		if err != nil {
			return err
		}
		c.Unset(capability.CAPS|capability.BOUNDS, drop...)

		if err := c.Apply(capability.CAPS | capability.BOUNDS); err != nil {
			return err
		}
	}
	return nil
}

// getCapabilitiesMask returns the specific cap mask values for the libcontainer types
func getCapabilitiesMask(container *libcontainer.Container) []capability.Cap {
	drop := []capability.Cap{}
	for key, enabled := range container.CapabilitiesMask {
		if !enabled {
			if c := libcontainer.GetCapability(key); c != nil {
				drop = append(drop, c.Value)
			}
		}
	}
	return drop
}
