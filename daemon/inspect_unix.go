// +build !windows

package daemon

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions/v1p19"
)

// This sets platform-specific fields
func setPlatformSpecificContainerFields(container *Container, contJSONBase *types.ContainerJSONBase) *types.ContainerJSONBase {
	contJSONBase.AppArmorProfile = container.AppArmorProfile
	contJSONBase.ResolvConfPath = container.ResolvConfPath
	contJSONBase.HostnamePath = container.HostnamePath
	contJSONBase.HostsPath = container.HostsPath

	return contJSONBase
}

// ContainerInspectPre120 gets containers for pre 1.20 APIs.
func (daemon *Daemon) ContainerInspectPre120(name string) (*v1p19.ContainerJSON, error) {
	container, err := daemon.Get(name)
	if err != nil {
		return nil, err
	}

	container.Lock()
	defer container.Unlock()

	base, err := daemon.getInspectData(container, false)
	if err != nil {
		return nil, err
	}

	volumes := make(map[string]string)
	volumesRW := make(map[string]bool)
	for _, m := range container.MountPoints {
		volumes[m.Destination] = m.Path()
		volumesRW[m.Destination] = m.RW
	}

	runConfig := types.RunConfig{
		Hostname:        container.Config.Hostname,
		Domainname:      container.Config.Domainname,
		User:            container.Config.User,
		AttachStdin:     container.Config.AttachStdin,
		AttachStdout:    container.Config.AttachStdout,
		AttachStderr:    container.Config.AttachStderr,
		ExposedPorts:    map[string]struct{}{},
		PublishService:  container.Config.PublishService,
		Tty:             container.Config.Tty,
		OpenStdin:       container.Config.OpenStdin,
		StdinOnce:       container.Config.StdinOnce,
		Env:             container.Config.Env,
		Cmd:             container.Config.Cmd.Slice(),
		Image:           container.Config.Image,
		Volumes:         container.Config.Volumes,
		WorkingDir:      container.Config.WorkingDir,
		Entrypoint:      container.Config.Entrypoint.Slice(),
		NetworkDisabled: container.Config.NetworkDisabled,
		MacAddress:      container.Config.MacAddress,
		OnBuild:         container.Config.OnBuild,
		Labels:          container.Config.Labels,
		StopSignal:      container.Config.StopSignal,
	}

	for k, v := range container.Config.ExposedPorts {
		runConfig.ExposedPorts[string(k)] = v
	}

	config := &v1p19.ContainerConfig{
		runConfig,
		container.hostConfig.VolumeDriver,
		container.hostConfig.Memory,
		container.hostConfig.MemorySwap,
		container.hostConfig.CPUShares,
		container.hostConfig.CpusetCpus,
	}

	return &v1p19.ContainerJSON{base, volumes, volumesRW, config}, nil
}

func addMountPoints(container *Container) []types.MountPoint {
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
