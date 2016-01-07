// +build windows

package daemon

import (
	"strings"

	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/daemon/execdriver/windows"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/layer"
	"github.com/docker/libnetwork"
)

func (daemon *Daemon) setupLinkedContainers(container *container.Container) ([]string, error) {
	return nil, nil
}

// updateContainerNetworkSettings update the network settings
func (daemon *Daemon) updateContainerNetworkSettings(container *container.Container) error {
	return nil
}

func (daemon *Daemon) initializeNetworking(container *container.Container) error {
	return nil
}

// ConnectToNetwork connects a container to the network
func (daemon *Daemon) ConnectToNetwork(container *container.Container, idOrName string) error {
	return nil
}

// DisconnectFromNetwork disconnects a container from the network.
func (daemon *Daemon) DisconnectFromNetwork(container *container.Container, n libnetwork.Network) error {
	return nil
}

func (daemon *Daemon) populateCommand(c *container.Container, env []string) error {
	en := &execdriver.Network{
		Interface: nil,
	}

	parts := strings.SplitN(string(c.HostConfig.NetworkMode), ":", 2)
	switch parts[0] {
	case "none":
	case "default", "": // empty string to support existing containers
		if !c.Config.NetworkDisabled {
			en.Interface = &execdriver.NetworkInterface{
				MacAddress:   c.Config.MacAddress,
				Bridge:       daemon.configStore.Bridge.VirtualSwitchName,
				PortBindings: c.HostConfig.PortBindings,

				// TODO Windows. Include IPAddress. There already is a
				// property IPAddress on execDrive.CommonNetworkInterface,
				// but there is no CLI option in docker to pass through
				// an IPAddress on docker run.
			}
		}
	default:
		return derr.ErrorCodeInvalidNetworkMode.WithArgs(c.HostConfig.NetworkMode)
	}

	// TODO Windows. More resource controls to be implemented later.
	resources := &execdriver.Resources{
		CommonResources: execdriver.CommonResources{
			CPUShares: c.HostConfig.CPUShares,
		},
	}

	processConfig := execdriver.ProcessConfig{
		CommonProcessConfig: execdriver.CommonProcessConfig{
			Entrypoint: c.Path,
			Arguments:  c.Args,
			Tty:        c.Config.Tty,
		},
		ConsoleSize: c.HostConfig.ConsoleSize,
	}

	processConfig.Env = env

	var layerPaths []string
	img, err := daemon.imageStore.Get(c.ImageID)
	if err != nil {
		return derr.ErrorCodeGetGraph.WithArgs(c.ImageID, err)
	}

	if img.RootFS != nil && img.RootFS.Type == "layers+base" {
		max := len(img.RootFS.DiffIDs)
		for i := 0; i <= max; i++ {
			img.RootFS.DiffIDs = img.RootFS.DiffIDs[:i]
			path, err := layer.GetLayerPath(daemon.layerStore, img.RootFS.ChainID())
			if err != nil {
				return derr.ErrorCodeGetLayer.WithArgs(err)
			}
			// Reverse order, expecting parent most first
			layerPaths = append([]string{path}, layerPaths...)
		}
	}

	m, err := c.RWLayer.Metadata()
	if err != nil {
		return derr.ErrorCodeGetLayerMetadata.WithArgs(err)
	}
	layerFolder := m["dir"]

	var hvPartition bool
	// Work out the isolation (whether it is a hypervisor partition)
	if c.HostConfig.Isolation.IsDefault() {
		// Not specified by caller. Take daemon default
		hvPartition = windows.DefaultIsolation.IsHyperV()
	} else {
		// Take value specified by caller
		hvPartition = c.HostConfig.Isolation.IsHyperV()
	}

	c.Command = &execdriver.Command{
		CommonCommand: execdriver.CommonCommand{
			ID:            c.ID,
			Rootfs:        c.BaseFS,
			InitPath:      "/.dockerinit",
			WorkingDir:    c.Config.WorkingDir,
			Network:       en,
			MountLabel:    c.GetMountLabel(),
			Resources:     resources,
			ProcessConfig: processConfig,
			ProcessLabel:  c.GetProcessLabel(),
		},
		FirstStart:  !c.HasBeenStartedBefore,
		LayerFolder: layerFolder,
		LayerPaths:  layerPaths,
		Hostname:    c.Config.Hostname,
		Isolation:   string(c.HostConfig.Isolation),
		ArgsEscaped: c.Config.ArgsEscaped,
		HvPartition: hvPartition,
	}

	return nil
}

// getSize returns real size & virtual size
func (daemon *Daemon) getSize(container *container.Container) (int64, int64) {
	// TODO Windows
	return 0, 0
}

// setNetworkNamespaceKey is a no-op on Windows.
func (daemon *Daemon) setNetworkNamespaceKey(containerID string, pid int) error {
	return nil
}

// allocateNetwork is a no-op on Windows.
func (daemon *Daemon) allocateNetwork(container *container.Container) error {
	return nil
}

func (daemon *Daemon) updateNetwork(container *container.Container) error {
	return nil
}

func (daemon *Daemon) releaseNetwork(container *container.Container) {
}

func (daemon *Daemon) setupIpcDirs(container *container.Container) error {
	return nil
}

// TODO Windows: Fix Post-TP4. This is a hack to allow docker cp to work
// against containers which have volumes. You will still be able to cp
// to somewhere on the container drive, but not to any mounted volumes
// inside the container. Without this fix, docker cp is broken to any
// container which has a volume, regardless of where the file is inside the
// container.
func (daemon *Daemon) mountVolumes(container *container.Container) error {
	return nil
}

func detachMounted(path string) error {
	return nil
}

func killProcessDirectly(container *container.Container) error {
	return nil
}
