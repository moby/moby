package container

import (
	"fmt"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
)

// Snapshot is a read only view for Containers
type Snapshot struct {
	ID           string `json:"Id"`
	Name         string
	Pid          int
	Managed      bool
	Image        string
	ImageID      string
	Command      string
	Ports        []types.Port
	ExposedPorts nat.PortSet
	PublishPorts nat.PortSet
	Labels       map[string]string
	State        string
	Status       string
	Health       string
	HostConfig   struct {
		NetworkMode string
		Isolation   string
	}
	NetworkSettings types.SummaryNetworkSettings
	Mounts          []types.MountPoint
	Created         time.Time
	StartedAt       time.Time
	Running         bool
	Paused          bool
	ExitCode        int
}

// Snapshot provides a read only view of a Container. Callers must hold a Lock on the container object.
func (container *Container) Snapshot() *Snapshot {
	snapshot := &Snapshot{
		ID:           container.ID,
		Name:         container.Name,
		Pid:          container.Pid,
		Managed:      container.Managed,
		ImageID:      container.ImageID.String(),
		Ports:        []types.Port{},
		ExposedPorts: make(nat.PortSet),
		PublishPorts: make(nat.PortSet),
		State:        container.State.StateString(),
		Status:       container.State.String(),
		Health:       container.State.HealthString(),
		Mounts:       container.GetMountPoints(),
		Created:      container.Created,
		StartedAt:    container.StartedAt,
		Running:      container.Running,
		Paused:       container.Paused,
		ExitCode:     container.ExitCode(),
	}

	if container.HostConfig != nil {
		snapshot.HostConfig.Isolation = string(container.HostConfig.Isolation)
		snapshot.HostConfig.NetworkMode = string(container.HostConfig.NetworkMode)
		for publish := range container.HostConfig.PortBindings {
			snapshot.PublishPorts[publish] = struct{}{}
		}
	}

	if container.Config != nil {
		snapshot.Image = container.Config.Image
		snapshot.Labels = container.Config.Labels
		for exposed := range container.Config.ExposedPorts {
			snapshot.ExposedPorts[exposed] = struct{}{}
		}
	}

	if len(container.Args) > 0 {
		args := []string{}
		for _, arg := range container.Args {
			if strings.Contains(arg, " ") {
				args = append(args, fmt.Sprintf("'%s'", arg))
			} else {
				args = append(args, arg)
			}
		}
		argsAsString := strings.Join(args, " ")
		snapshot.Command = fmt.Sprintf("%s %s", container.Path, argsAsString)
	} else {
		snapshot.Command = container.Path
	}

	if container.NetworkSettings != nil {
		networks := make(map[string]*network.EndpointSettings)
		for name, netw := range container.NetworkSettings.Networks {
			if netw == nil || netw.EndpointSettings == nil {
				continue
			}
			networks[name] = &network.EndpointSettings{
				EndpointID:          netw.EndpointID,
				Gateway:             netw.Gateway,
				IPAddress:           netw.IPAddress,
				IPPrefixLen:         netw.IPPrefixLen,
				IPv6Gateway:         netw.IPv6Gateway,
				GlobalIPv6Address:   netw.GlobalIPv6Address,
				GlobalIPv6PrefixLen: netw.GlobalIPv6PrefixLen,
				MacAddress:          netw.MacAddress,
				NetworkID:           netw.NetworkID,
			}
			if netw.IPAMConfig != nil {
				networks[name].IPAMConfig = &network.EndpointIPAMConfig{
					IPv4Address: netw.IPAMConfig.IPv4Address,
					IPv6Address: netw.IPAMConfig.IPv6Address,
				}
			}
		}
		snapshot.NetworkSettings = types.SummaryNetworkSettings{Networks: networks}
		for port, bindings := range container.NetworkSettings.Ports {
			p, err := nat.ParsePort(port.Port())
			if err != nil {
				logrus.Warnf("invalid port map %+v", err)
				continue
			}
			if len(bindings) == 0 {
				snapshot.Ports = append(snapshot.Ports, types.Port{
					PrivatePort: uint16(p),
					Type:        port.Proto(),
				})
				continue
			}
			for _, binding := range bindings {
				h, err := nat.ParsePort(binding.HostPort)
				if err != nil {
					logrus.Warnf("invalid host port map %+v", err)
					continue
				}
				snapshot.Ports = append(snapshot.Ports, types.Port{
					PrivatePort: uint16(p),
					PublicPort:  uint16(h),
					Type:        port.Proto(),
					IP:          binding.HostIP,
				})
			}
		}

	}

	return snapshot
}
