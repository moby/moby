package daemon

import (
	"fmt"
	"strings"
	"syscall"

	"github.com/docker/docker/container"
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
		// TODO We don't yet have the ImagePath hooked up. But set to
		// something non-nil to pickup in libcontainerd.
		s.Windows.HvRuntime = &windowsoci.HvRuntime{}
	}

	// In s.Process
	if c.Config.ArgsEscaped {
		s.Process.Args = append([]string{c.Path}, c.Args...)
	} else {
		// TODO (jstarks): escape the entrypoint too once the tests are fixed to not rely on this behavior
		s.Process.Args = append([]string{c.Path}, escapeArgs(c.Args)...)
	}
	s.Process.Cwd = c.Config.WorkingDir
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
	if img.RootFS != nil && img.RootFS.Type == "layers+base" {
		max := len(img.RootFS.DiffIDs)
		for i := 0; i <= max; i++ {
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

	// In s.Windows.Networking (TP5+ libnetwork way of doing things)
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

	// In s.Windows.Networking (TP4 back compat)
	// TODO Windows: Post TP4 - Remove this along with definitions from spec
	// and changes to libcontainerd to not read these fields.
	if daemon.netController == nil {
		parts := strings.SplitN(string(c.HostConfig.NetworkMode), ":", 2)
		switch parts[0] {
		case "none":
		case "default", "": // empty string to support existing containers
			if !c.Config.NetworkDisabled {
				s.Windows.Networking = &windowsoci.Networking{
					MacAddress:   c.Config.MacAddress,
					Bridge:       daemon.configStore.bridgeConfig.Iface,
					PortBindings: c.HostConfig.PortBindings,
				}
			}
		default:
			return nil, fmt.Errorf("invalid network mode: %s", c.HostConfig.NetworkMode)
		}
	}

	// In s.Windows.Resources
	// @darrenstahlmsft implement these resources
	cpuShares := uint64(c.HostConfig.CPUShares)
	s.Windows.Resources = &windowsoci.Resources{
		CPU: &windowsoci.CPU{
			//TODO Count: ...,
			//TODO Percent: ...,
			Shares: &cpuShares,
		},
		Memory: &windowsoci.Memory{
		//TODO Limit: ...,
		//TODO Reservation: ...,
		},
		Network: &windowsoci.Network{
		//TODO Bandwidth: ...,
		},
		Storage: &windowsoci.Storage{
		//TODO Bps: ...,
		//TODO Iops: ...,
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
