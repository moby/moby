// +build !windows

package decorators

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon"
)

// setPlatformSpecificContainerFields sets fields only available in unix hosts.
func setPlatformSpecificContainerFields(container *daemon.Container, contJSONBase *types.ContainerJSONBase) *types.ContainerJSONBase {
	contJSONBase.AppArmorProfile = container.AppArmorProfile
	contJSONBase.ResolvConfPath = container.ResolvConfPath
	contJSONBase.HostnamePath = container.HostnamePath
	contJSONBase.HostsPath = container.HostsPath

	return contJSONBase
}

// getMountPoints transforms mount points to be serialized by the API.
func getMountPoints(container *daemon.Container) []types.MountPoint {
	mountPoints := make([]types.MountPoint, 0, len(container.MountPoints))
	for _, m := range container.MountPoints {
		mountPoints = append(mountPoints, types.MountPoint{
			Name:        m.Name,
			Source:      m.Path(),
			Destination: m.Destination,
			Driver:      m.Driver,
			Mode:        m.Mode,
			RW:          m.RW,
		})
	}
	return mountPoints
}
