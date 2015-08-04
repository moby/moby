// +build windows

package daemon

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/daemon/graphdriver/windows"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/archive"
	"github.com/microsoft/hcsshim"
)

// This is deliberately empty on Windows as the default path will be set by
// the container. Docker has no context of what the default path should be.
const DefaultPathEnv = ""

// Container holds fields specific to the Windows implementation. See
// CommonContainer for standard fields common to all containers.
type Container struct {
	CommonContainer

	// Fields below here are platform specific.
}

func killProcessDirectly(container *Container) error {
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
		Mtu:       c.daemon.config.Mtu,
		Interface: nil,
	}

	parts := strings.SplitN(string(c.hostConfig.NetworkMode), ":", 2)
	switch parts[0] {

	case "none":
	case "default", "": // empty string to support existing containers
		if !c.Config.NetworkDisabled {
			en.Interface = &execdriver.NetworkInterface{
				MacAddress: c.Config.MacAddress,
				Bridge:     c.daemon.config.Bridge.VirtualSwitchName,
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

	var layerFolder string
	var layerPaths []string

	// The following is specific to the Windows driver. We do this to
	// enable VFS to continue operating for development purposes.
	if wd, ok := c.daemon.driver.(*windows.WindowsGraphDriver); ok {
		var err error
		var img *image.Image
		var ids []string

		if img, err = c.daemon.graph.Get(c.ImageID); err != nil {
			return fmt.Errorf("Failed to graph.Get on ImageID %s - %s", c.ImageID, err)
		}
		if ids, err = c.daemon.graph.ParentLayerIds(img); err != nil {
			return fmt.Errorf("Failed to get parentlayer ids %s", img.ID)
		}
		layerPaths = wd.LayerIdsToPaths(ids)
		layerFolder = filepath.Join(wd.Info().HomeDir, filepath.Base(c.ID))
	}

	// TODO Windows: Factor out remainder of unused fields.
	c.command = &execdriver.Command{
		ID:             c.ID,
		Rootfs:         c.rootfsPath(),
		ReadonlyRootfs: c.hostConfig.ReadonlyRootfs,
		InitPath:       "/.dockerinit",
		WorkingDir:     c.Config.WorkingDir,
		Network:        en,
		Pid:            pid,
		Resources:      resources,
		CapAdd:         c.hostConfig.CapAdd.Slice(),
		CapDrop:        c.hostConfig.CapDrop.Slice(),
		ProcessConfig:  processConfig,
		ProcessLabel:   c.getProcessLabel(),
		MountLabel:     c.getMountLabel(),
		FirstStart:     !c.HasBeenStartedBefore,
		LayerFolder:    layerFolder,
		LayerPaths:     layerPaths,
	}

	return nil
}

// GetSize returns real size & virtual size
func (container *Container) getSize() (int64, int64) {
	// TODO Windows
	return 0, 0
}

// allocateNetwork is a no-op on Windows.
func (container *Container) allocateNetwork() error {
	return nil
}

func (container *Container) exportRw() (archive.Archive, error) {
	if container.IsRunning() {
		return nil, fmt.Errorf("Cannot export a running container.")
	}
	// TODO Windows. Implementation (different to Linux)
	return nil, nil
}

func (container *Container) updateNetwork() error {
	return nil
}

func (container *Container) releaseNetwork() {
}

func (container *Container) unmountVolumes(forceSyscall bool) error {
	return nil
}

// PrepareStorage prepares the layer to boot using the windows driver.
func (container *Container) PrepareStorage() error {
	if wd, ok := container.daemon.driver.(*windows.WindowsGraphDriver); ok {
		// Get list of paths to parent layers.
		var ids []string
		if container.ImageID != "" {
			img, err := container.daemon.graph.Get(container.ImageID)
			if err != nil {
				return err
			}

			ids, err = container.daemon.graph.ParentLayerIds(img)
			if err != nil {
				return err
			}
		}

		if err := hcsshim.PrepareLayer(wd.Info(), container.ID, wd.LayerIdsToPaths(ids)); err != nil {
			return err
		}
	}
	return nil
}

// CleanupStorage unprepares the layer after shutdown? FIXME
func (container *Container) CleanupStorage() error {
	if wd, ok := container.daemon.driver.(*windows.WindowsGraphDriver); ok {
		return hcsshim.UnprepareLayer(wd.Info(), container.ID)
	}
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
