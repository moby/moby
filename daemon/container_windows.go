// +build windows

package daemon

import (
	"fmt"
	"strings"

	"github.com/docker/docker/daemon/execdriver"
)

// This is deliberately empty on Windows as the default path will be set by
// the container. Docker has no context of what the default path should be.
const DefaultPathEnv = ""

type Container struct {
	CommonContainer

	// Fields below here are platform specific.
}

func killProcessDirectly(container *Container) error {
	return nil
}

func (container *Container) setupContainerDns() error {
	return nil
}

func (container *Container) updateParentsHosts() error {
	return nil
}

func (container *Container) setupLinkedContainers() ([]string, error) {
	return nil, nil
}

func (container *Container) createDaemonEnvironment(linkedEnv []string) []string {
	// On Windows, nothing to link. Just return the container environment.
	return container.Config.Env
}

func (container *Container) initializeNetworking() error {
	return nil
}

func (container *Container) setupWorkingDirectory() error {
	return nil
}

func populateCommand(c *Container, env []string) error {
	en := &execdriver.Network{
		Interface: nil,
	}

	parts := strings.SplitN(string(c.hostConfig.NetworkMode), ":", 2)
	switch parts[0] {
	case "none":
	case "default", "": // empty string to support existing containers
		if !c.Config.NetworkDisabled {
			en.Interface = &execdriver.NetworkInterface{
				MacAddress:   c.Config.MacAddress,
				Bridge:       c.daemon.config.Bridge.VirtualSwitchName,
				PortBindings: c.hostConfig.PortBindings,

				// TODO Windows. Include IPAddress. There already is a
				// property IPAddress on execDrive.CommonNetworkInterface,
				// but there is no CLI option in docker to pass through
				// an IPAddress on docker run.
			}
		}
	default:
		return fmt.Errorf("invalid network mode: %s", c.hostConfig.NetworkMode)
	}

	pid := &execdriver.Pid{}

	// TODO Windows. This can probably be factored out.
	pid.HostPid = c.hostConfig.PidMode.IsHost()

	// TODO Windows. Resource controls to be implemented later.
	resources := &execdriver.Resources{}

	// TODO Windows. Further refactoring required (privileged/user)
	processConfig := execdriver.ProcessConfig{
		Privileged:  c.hostConfig.Privileged,
		Entrypoint:  c.Path,
		Arguments:   c.Args,
		Tty:         c.Config.Tty,
		User:        c.Config.User,
		ConsoleSize: c.hostConfig.ConsoleSize,
	}

	processConfig.Env = env

	var layerPaths []string
	img, err := c.daemon.graph.Get(c.ImageID)
	if err != nil {
		return fmt.Errorf("Failed to graph.Get on ImageID %s - %s", c.ImageID, err)
	}
	for i := img; i != nil && err == nil; i, err = c.daemon.graph.GetParent(i) {
		lp, err := c.daemon.driver.Get(i.ID, "")
		if err != nil {
			return fmt.Errorf("Failed to get layer path from graphdriver %s for ImageID %s - %s", c.daemon.driver.String(), i.ID, err)
		}
		layerPaths = append(layerPaths, lp)
		err = c.daemon.driver.Put(i.ID)
		if err != nil {
			return fmt.Errorf("Failed to put layer path from graphdriver %s for ImageID %s - %s", c.daemon.driver.String(), i.ID, err)
		}
	}
	m, err := c.daemon.driver.GetMetadata(c.ID)
	if err != nil {
		return fmt.Errorf("Failed to get layer metadata - %s", err)
	}
	layerFolder := m["dir"]

	// TODO Windows: Factor out remainder of unused fields.
	c.command = &execdriver.Command{
		ID:             c.ID,
		Rootfs:         c.RootfsPath(),
		ReadonlyRootfs: c.hostConfig.ReadonlyRootfs,
		InitPath:       "/.dockerinit",
		WorkingDir:     c.Config.WorkingDir,
		Network:        en,
		Pid:            pid,
		Resources:      resources,
		CapAdd:         c.hostConfig.CapAdd.Slice(),
		CapDrop:        c.hostConfig.CapDrop.Slice(),
		ProcessConfig:  processConfig,
		ProcessLabel:   c.GetProcessLabel(),
		MountLabel:     c.GetMountLabel(),
		FirstStart:     !c.HasBeenStartedBefore,
		LayerFolder:    layerFolder,
		LayerPaths:     layerPaths,
	}

	return nil
}

// GetSize, return real size, virtual size
func (container *Container) GetSize() (int64, int64) {
	// TODO Windows
	return 0, 0
}

func (container *Container) AllocateNetwork() error {
	return nil
}

func (container *Container) UpdateNetwork() error {
	return nil
}

func (container *Container) ReleaseNetwork() {
}

func (container *Container) RestoreNetwork() error {
	return nil
}

func (container *Container) UnmountVolumes(forceSyscall bool) error {
	return nil
}

// prepareMountPoints is a no-op on Windows
func (container *Container) prepareMountPoints() error {
	return nil
}

// removeMountPoints is a no-op on Windows.
func (container *Container) removeMountPoints() error {
	return nil
}

func (container *Container) setupIpcDirs() error {
	return nil
}

func (container *Container) unmountIpcMounts() error {
	return nil
}

func (container *Container) ipcMounts() []execdriver.Mount {
	return nil
}
