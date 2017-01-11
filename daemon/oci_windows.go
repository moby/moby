package daemon

import (
	"syscall"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/oci"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func (daemon *Daemon) createSpec(c *container.Container) (*specs.Spec, error) {
	s := oci.DefaultSpec()

	linkedEnv, err := daemon.setupLinkedContainers(c)
	if err != nil {
		return nil, err
	}

	if err := c.SetupWorkingDirectory(0, 0); err != nil {
		return nil, err
	}

	// In base spec
	s.Hostname = c.FullHostname()

	// In s.Mounts
	mounts, err := daemon.setupMounts(c)
	if err != nil {
		return nil, err
	}
	for _, mount := range mounts {
		m := specs.Mount{
			Source:      mount.Source,
			Destination: mount.Destination,
		}
		if !mount.Writable {
			m.Options = append(m.Options, "ro")
		}
		s.Mounts = append(s.Mounts, m)
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
	s.Process.Env = c.CreateDaemonEnvironment(c.Config.Tty, linkedEnv)
	s.Process.ConsoleSize.Height = c.HostConfig.ConsoleSize[0]
	s.Process.ConsoleSize.Width = c.HostConfig.ConsoleSize[1]
	s.Process.Terminal = c.Config.Tty
	s.Process.User.Username = c.Config.User

	// In spec.Root. This is not set for Hyper-V containers
	isHyperV := false
	if c.HostConfig.Isolation.IsDefault() {
		// Container using default isolation, so take the default from the daemon configuration
		isHyperV = daemon.defaultIsolation.IsHyperV()
	} else {
		// Container may be requesting an explicit isolation mode.
		isHyperV = c.HostConfig.Isolation.IsHyperV()
	}
	if !isHyperV {
		s.Root.Path = c.BaseFS
	}
	s.Root.Readonly = false // Windows does not support a read-only root filesystem

	// In s.Windows.Resources
	// @darrenstahlmsft implement these resources
	cpuShares := uint16(c.HostConfig.CPUShares)
	cpuPercent := uint8(c.HostConfig.CPUPercent)
	if c.HostConfig.NanoCPUs > 0 {
		cpuPercent = uint8(c.HostConfig.NanoCPUs * 100 / int64(sysinfo.NumCPU()) / 1e9)
	}
	cpuCount := uint64(c.HostConfig.CPUCount)
	memoryLimit := uint64(c.HostConfig.Memory)
	s.Windows.Resources = &specs.WindowsResources{
		CPU: &specs.WindowsCPUResources{
			Percent: &cpuPercent,
			Shares:  &cpuShares,
			Count:   &cpuCount,
		},
		Memory: &specs.WindowsMemoryResources{
			Limit: &memoryLimit,
			//TODO Reservation: ...,
		},
		Network: &specs.WindowsNetworkResources{
		//TODO Bandwidth: ...,
		},
		Storage: &specs.WindowsStorageResources{
			Bps:  &c.HostConfig.IOMaximumBandwidth,
			Iops: &c.HostConfig.IOMaximumIOps,
		},
	}
	return (*specs.Spec)(&s), nil
}

func escapeArgs(args []string) []string {
	escapedArgs := make([]string, len(args))
	for i, a := range args {
		escapedArgs[i] = syscall.EscapeArg(a)
	}
	return escapedArgs
}

// mergeUlimits merge the Ulimits from HostConfig with daemon defaults, and update HostConfig
// It will do nothing on non-Linux platform
func (daemon *Daemon) mergeUlimits(c *containertypes.HostConfig) {
	return
}
