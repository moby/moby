package capabilities

import (
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/syndtr/gocapability/capability"
	"os"
)

// DropCapabilities drops capabilities for the current process based
// on the container's configuration.
func DropCapabilities(container *libcontainer.Container) error {
	if drop := getCapabilities(container); len(drop) > 0 {
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

// getCapabilities returns the specific cap values for the libcontainer types
func getCapabilities(container *libcontainer.Container) []capability.Cap {
	drop := []capability.Cap{}
	for _, c := range container.Capabilities {
		drop = append(drop, c.Value)
	}
	return drop
}
