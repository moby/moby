package capabilities

import (
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/syndtr/gocapability/capability"
	"os"
)

var capMap = map[libcontainer.Capability]capability.Cap{
	libcontainer.CAP_SETPCAP:        capability.CAP_SETPCAP,
	libcontainer.CAP_SYS_MODULE:     capability.CAP_SYS_MODULE,
	libcontainer.CAP_SYS_RAWIO:      capability.CAP_SYS_RAWIO,
	libcontainer.CAP_SYS_PACCT:      capability.CAP_SYS_PACCT,
	libcontainer.CAP_SYS_ADMIN:      capability.CAP_SYS_ADMIN,
	libcontainer.CAP_SYS_NICE:       capability.CAP_SYS_NICE,
	libcontainer.CAP_SYS_RESOURCE:   capability.CAP_SYS_RESOURCE,
	libcontainer.CAP_SYS_TIME:       capability.CAP_SYS_TIME,
	libcontainer.CAP_SYS_TTY_CONFIG: capability.CAP_SYS_TTY_CONFIG,
	libcontainer.CAP_MKNOD:          capability.CAP_MKNOD,
	libcontainer.CAP_AUDIT_WRITE:    capability.CAP_AUDIT_WRITE,
	libcontainer.CAP_AUDIT_CONTROL:  capability.CAP_AUDIT_CONTROL,
	libcontainer.CAP_MAC_OVERRIDE:   capability.CAP_MAC_OVERRIDE,
	libcontainer.CAP_MAC_ADMIN:      capability.CAP_MAC_ADMIN,
}

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

func getCapabilities(container *libcontainer.Container) []capability.Cap {
	drop := []capability.Cap{}
	for _, c := range container.Capabilities {
		drop = append(drop, capMap[c])
	}
	return drop
}
