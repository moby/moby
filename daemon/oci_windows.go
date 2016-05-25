package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/docker/docker/container"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/libcontainerd/windowsoci"
	"github.com/docker/docker/oci"
)

func (daemon *Daemon) createSpec(c *container.Container) (*libcontainerd.Spec, error) {
	s := oci.DefaultSpec()

	linkedEnv, err := daemon.setupLinkedContainers(c)
	if err != nil {
		return nil, err
	}

	// TODO Windows - this can be removed. Not used (UID/GID)
	rootUID, rootGID := daemon.GetRemappedUIDGID()
	if err := c.SetupWorkingDirectory(rootUID, rootGID); err != nil {
		return nil, err
	}

	img, err := daemon.imageStore.Get(c.ImageID)
	if err != nil {
		return nil, fmt.Errorf("Failed to graph.Get on ImageID %s - %s", c.ImageID, err)
	}

	s.Platform.OSVersion = img.OSVersion

	// In base spec
	s.Hostname = c.FullHostname()

	// In s.Mounts
	mounts, err := daemon.setupMounts(c)
	if err != nil {
		return nil, err
	}
	for _, mount := range mounts {
		s.Mounts = append(s.Mounts, windowsoci.Mount{
			Source:      mount.Source,
			Destination: mount.Destination,
			Readonly:    !mount.Writable,
		})
	}

	// In s.Process
	s.Process.Args = append([]string{c.Path}, c.Args...)
	if !c.Config.ArgsEscaped {
		s.Process.Args = escapeArgs(s.Process.Args)
	}
	s.Process.Cwd = c.Config.WorkingDir
	if len(s.Process.Cwd) == 0 {
		// We default to C:\ to workaround the oddity of the case that the
		// default directory for cmd running as LocalSystem (or
		// ContainerAdministrator) is c:\windows\system32. Hence docker run
		// <image> cmd will by default end in c:\windows\system32, rather
		// than 'root' (/) on Linux. The oddity is that if you have a dockerfile
		// which has no WORKDIR and has a COPY file ., . will be interpreted
		// as c:\. Hence, setting it to default of c:\ makes for consistency.
		s.Process.Cwd = `C:\`
	}
	s.Process.Env = c.CreateDaemonEnvironment(linkedEnv)
	s.Process.InitialConsoleSize = c.HostConfig.ConsoleSize
	s.Process.Terminal = c.Config.Tty
	s.Process.User.User = c.Config.User

	// In spec.Root
	s.Root.Path = c.BaseFS
	s.Root.Readonly = c.HostConfig.ReadonlyRootfs

	// In s.Windows
	s.Windows.FirstStart = !c.HasBeenStartedBefore

	// s.Windows.LayerFolder.
	m, err := c.RWLayer.Metadata()
	if err != nil {
		return nil, fmt.Errorf("Failed to get layer metadata - %s", err)
	}
	s.Windows.LayerFolder = m["dir"]

	// s.Windows.LayerPaths
	var layerPaths []string
	if img.RootFS != nil && (img.RootFS.Type == image.TypeLayers || img.RootFS.Type == image.TypeLayersWithBase) {
		// Get the layer path for each layer.
		start := 1
		if img.RootFS.Type == image.TypeLayersWithBase {
			// Include an empty slice to get the base layer ID.
			start = 0
		}
		max := len(img.RootFS.DiffIDs)
		for i := start; i <= max; i++ {
			img.RootFS.DiffIDs = img.RootFS.DiffIDs[:i]
			path, err := layer.GetLayerPath(daemon.layerStore, img.RootFS.ChainID())
			if err != nil {
				return nil, fmt.Errorf("Failed to get layer path from graphdriver %s for ImageID %s - %s", daemon.layerStore, img.RootFS.ChainID(), err)
			}
			// Reverse order, expecting parent most first
			layerPaths = append([]string{path}, layerPaths...)
		}
	}
	s.Windows.LayerPaths = layerPaths

	// Are we going to run as a Hyper-V container?
	hv := false
	if c.HostConfig.Isolation.IsDefault() {
		// Container is set to use the default, so take the default from the daemon configuration
		hv = daemon.defaultIsolation.IsHyperV()
	} else {
		// Container is requesting an isolation mode. Honour it.
		hv = c.HostConfig.Isolation.IsHyperV()
	}
	if hv {
		hvr := &windowsoci.HvRuntime{}
		if img.RootFS != nil && img.RootFS.Type == image.TypeLayers {
			// For TP5, the utility VM is part of the base layer.
			// TODO-jstarks: Add support for separate utility VM images
			// once it is decided how they can be stored.
			uvmpath := filepath.Join(layerPaths[len(layerPaths)-1], "UtilityVM")
			_, err = os.Stat(uvmpath)
			if err != nil {
				if os.IsNotExist(err) {
					err = errors.New("container image does not contain a utility VM")
				}
				return nil, err
			}

			hvr.ImagePath = uvmpath
		}

		s.Windows.HvRuntime = hvr
	}

	// In s.Windows.Networking
	// Connect all the libnetwork allocated networks to the container
	var epList []string
	if c.NetworkSettings != nil {
		for n := range c.NetworkSettings.Networks {
			sn, err := daemon.FindNetwork(n)
			if err != nil {
				continue
			}

			ep, err := c.GetEndpointInNetwork(sn)
			if err != nil {
				continue
			}

			data, err := ep.DriverInfo()
			if err != nil {
				continue
			}
			if data["hnsid"] != nil {
				epList = append(epList, data["hnsid"].(string))
			}
		}
	}
	s.Windows.Networking = &windowsoci.Networking{
		EndpointList: epList,
	}

	// In s.Windows.Resources
	// @darrenstahlmsft implement these resources
	cpuShares := uint64(c.HostConfig.CPUShares)
	s.Windows.Resources = &windowsoci.Resources{
		CPU: &windowsoci.CPU{
			Percent: &c.HostConfig.CPUPercent,
			Shares:  &cpuShares,
		},
		Memory: &windowsoci.Memory{
		//TODO Limit: ...,
		//TODO Reservation: ...,
		},
		Network: &windowsoci.Network{
		//TODO Bandwidth: ...,
		},
		Storage: &windowsoci.Storage{
			Bps:  &c.HostConfig.IOMaximumBandwidth,
			Iops: &c.HostConfig.IOMaximumIOps,
			//TODO SandboxSize: ...,
		},
	}
	return (*libcontainerd.Spec)(&s), nil
}

func escapeArgs(args []string) []string {
	escapedArgs := make([]string, len(args))
	for i, a := range args {
		escapedArgs[i] = syscall.EscapeArg(a)
	}
	return escapedArgs
}
