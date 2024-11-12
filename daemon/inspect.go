// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.22

package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/backend"
	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/daemon/network"
	"github.com/docker/docker/errdefs"
	"github.com/docker/go-connections/nat"
)

// ContainerInspect returns low-level information about a
// container. Returns an error if the container cannot be found, or if
// there is an error getting the data.
func (daemon *Daemon) ContainerInspect(ctx context.Context, name string, options backend.ContainerInspectOptions) (*containertypes.InspectResponse, error) {
	ctr, err := daemon.GetContainer(name)
	if err != nil {
		return nil, err
	}

	ctr.Lock()

	base, err := daemon.getInspectData(&daemon.config().Config, ctr)
	if err != nil {
		ctr.Unlock()
		return nil, err
	}

	apiNetworks := make(map[string]*networktypes.EndpointSettings)
	for nwName, epConf := range ctr.NetworkSettings.Networks {
		if epConf.EndpointSettings != nil {
			// We must make a copy of this pointer object otherwise it can race with other operations
			apiNetworks[nwName] = epConf.EndpointSettings.Copy()
		}
	}

	mountPoints := ctr.GetMountPoints()
	networkSettings := &containertypes.NetworkSettings{
		NetworkSettingsBase: containertypes.NetworkSettingsBase{
			Bridge:                 ctr.NetworkSettings.Bridge,
			SandboxID:              ctr.NetworkSettings.SandboxID,
			SandboxKey:             ctr.NetworkSettings.SandboxKey,
			HairpinMode:            ctr.NetworkSettings.HairpinMode,
			LinkLocalIPv6Address:   ctr.NetworkSettings.LinkLocalIPv6Address,
			LinkLocalIPv6PrefixLen: ctr.NetworkSettings.LinkLocalIPv6PrefixLen,
			SecondaryIPAddresses:   ctr.NetworkSettings.SecondaryIPAddresses,
			SecondaryIPv6Addresses: ctr.NetworkSettings.SecondaryIPv6Addresses,
		},
		DefaultNetworkSettings: getDefaultNetworkSettings(ctr.NetworkSettings.Networks),
		Networks:               apiNetworks,
	}

	ports := make(nat.PortMap, len(ctr.NetworkSettings.Ports))
	for k, pm := range ctr.NetworkSettings.Ports {
		ports[k] = pm
	}
	networkSettings.NetworkSettingsBase.Ports = ports

	ctr.Unlock()

	if options.Size {
		sizeRw, sizeRootFs, err := daemon.imageService.GetContainerLayerSize(ctx, base.ID)
		if err != nil {
			return nil, err
		}
		base.SizeRw = &sizeRw
		base.SizeRootFs = &sizeRootFs
	}

	return &containertypes.InspectResponse{
		ContainerJSONBase: base,
		Mounts:            mountPoints,
		Config:            ctr.Config,
		NetworkSettings:   networkSettings,
	}, nil
}

func (daemon *Daemon) getInspectData(daemonCfg *config.Config, container *container.Container) (*containertypes.ContainerJSONBase, error) {
	// make a copy to play with
	hostConfig := *container.HostConfig

	children := daemon.children(container)
	hostConfig.Links = nil // do not expose the internal structure
	for linkAlias, child := range children {
		hostConfig.Links = append(hostConfig.Links, fmt.Sprintf("%s:%s", child.Name, linkAlias))
	}

	// We merge the Ulimits from hostConfig with daemon default
	daemon.mergeUlimits(&hostConfig, daemonCfg)

	// Migrate the container's default network's MacAddress to the top-level
	// Config.MacAddress field for older API versions (< 1.44). We set it here
	// unconditionally, to keep backward compatibility with clients that use
	// unversioned API endpoints.
	if container.Config != nil && container.Config.MacAddress == "" { //nolint:staticcheck // ignore SA1019: field is deprecated, but still used on API < v1.44.
		if nwm := hostConfig.NetworkMode; nwm.IsBridge() || nwm.IsUserDefined() {
			if epConf, ok := container.NetworkSettings.Networks[nwm.NetworkName()]; ok {
				container.Config.MacAddress = epConf.DesiredMacAddress //nolint:staticcheck // ignore SA1019: field is deprecated, but still used on API < v1.44.
			}
		}
	}

	var containerHealth *containertypes.Health
	if container.State.Health != nil {
		containerHealth = &containertypes.Health{
			Status:        container.State.Health.Status(),
			FailingStreak: container.State.Health.FailingStreak,
			Log:           append([]*containertypes.HealthcheckResult{}, container.State.Health.Log...),
		}
	}

	containerState := &containertypes.State{
		Status:     container.State.StateString(),
		Running:    container.State.Running,
		Paused:     container.State.Paused,
		Restarting: container.State.Restarting,
		OOMKilled:  container.State.OOMKilled,
		Dead:       container.State.Dead,
		Pid:        container.State.Pid,
		ExitCode:   container.State.ExitCode(),
		Error:      container.State.ErrorMsg,
		StartedAt:  container.State.StartedAt.Format(time.RFC3339Nano),
		FinishedAt: container.State.FinishedAt.Format(time.RFC3339Nano),
		Health:     containerHealth,
	}

	contJSONBase := &containertypes.ContainerJSONBase{
		ID:           container.ID,
		Created:      container.Created.Format(time.RFC3339Nano),
		Path:         container.Path,
		Args:         container.Args,
		State:        containerState,
		Image:        container.ImageID.String(),
		LogPath:      container.LogPath,
		Name:         container.Name,
		RestartCount: container.RestartCount,
		Driver:       container.Driver,
		Platform:     container.OS,
		MountLabel:   container.MountLabel,
		ProcessLabel: container.ProcessLabel,
		ExecIDs:      container.GetExecIDs(),
		HostConfig:   &hostConfig,
	}

	// Now set any platform-specific fields
	contJSONBase = setPlatformSpecificContainerFields(container, contJSONBase)

	contJSONBase.GraphDriver.Name = container.Driver

	if daemon.UsesSnapshotter() {
		// Additional information only applies to graphDrivers, so we're done.
		return contJSONBase, nil
	}

	if container.RWLayer == nil {
		if container.Dead {
			return contJSONBase, nil
		}
		return nil, errdefs.System(errors.New("RWLayer of container " + container.ID + " is unexpectedly nil"))
	}

	graphDriverData, err := container.RWLayer.Metadata()
	if err != nil {
		if container.Dead {
			// container is marked as Dead, and its graphDriver metadata may
			// have been removed; we can ignore errors.
			return contJSONBase, nil
		}
		return nil, errdefs.System(err)
	}

	contJSONBase.GraphDriver.Data = graphDriverData
	return contJSONBase, nil
}

// ContainerExecInspect returns low-level information about the exec
// command. An error is returned if the exec cannot be found.
func (daemon *Daemon) ContainerExecInspect(id string) (*backend.ExecInspect, error) {
	e := daemon.execCommands.Get(id)
	if e == nil {
		return nil, errExecNotFound(id)
	}

	if ctr := daemon.containers.Get(e.Container.ID); ctr == nil {
		return nil, errExecNotFound(id)
	}

	e.Lock()
	defer e.Unlock()
	pc := inspectExecProcessConfig(e)
	var pid int
	if e.Process != nil {
		pid = int(e.Process.Pid())
	}

	return &backend.ExecInspect{
		ID:            e.ID,
		Running:       e.Running,
		ExitCode:      e.ExitCode,
		ProcessConfig: pc,
		OpenStdin:     e.OpenStdin,
		OpenStdout:    e.OpenStdout,
		OpenStderr:    e.OpenStderr,
		CanRemove:     e.CanRemove,
		ContainerID:   e.Container.ID,
		DetachKeys:    e.DetachKeys,
		Pid:           pid,
	}, nil
}

// getDefaultNetworkSettings creates the deprecated structure that holds the information
// about the bridge network for a container.
func getDefaultNetworkSettings(networks map[string]*network.EndpointSettings) containertypes.DefaultNetworkSettings {
	nw, ok := networks[networktypes.NetworkBridge]
	if !ok || nw.EndpointSettings == nil {
		return containertypes.DefaultNetworkSettings{}
	}

	return containertypes.DefaultNetworkSettings{
		EndpointID:          nw.EndpointSettings.EndpointID,
		Gateway:             nw.EndpointSettings.Gateway,
		GlobalIPv6Address:   nw.EndpointSettings.GlobalIPv6Address,
		GlobalIPv6PrefixLen: nw.EndpointSettings.GlobalIPv6PrefixLen,
		IPAddress:           nw.EndpointSettings.IPAddress,
		IPPrefixLen:         nw.EndpointSettings.IPPrefixLen,
		IPv6Gateway:         nw.EndpointSettings.IPv6Gateway,
		MacAddress:          nw.EndpointSettings.MacAddress,
	}
}
