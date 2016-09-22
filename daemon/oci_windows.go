package daemon

import (
	"syscall"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
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

	// In base spec
	s.Hostname = c.FullHostname()

	// In s.Mounts
	mounts, err := daemon.setupMounts(c)
	if err != nil {
		return nil, err
	}
	for _, mount := range mounts {
		m := windowsoci.Mount{
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
	s.Process.Env = c.CreateDaemonEnvironment(linkedEnv)
	s.Process.ConsoleSize.Height = c.HostConfig.ConsoleSize[0]
	s.Process.ConsoleSize.Width = c.HostConfig.ConsoleSize[1]
	s.Process.Terminal = c.Config.Tty
	s.Process.User.Username = c.Config.User

	// In spec.Root
	s.Root.Path = c.BaseFS
	s.Root.Readonly = c.HostConfig.ReadonlyRootfs

	// In s.Windows.Resources
	// @darrenstahlmsft implement these resources
	cpuShares := uint64(c.HostConfig.CPUShares)
	s.Windows.Resources = &windowsoci.WindowsResources{
		CPU: &windowsoci.WindowsCPU{
			Percent: &c.HostConfig.CPUPercent,
			Shares:  &cpuShares,
		},
		Memory: &windowsoci.WindowsMemory{
			Limit: &c.HostConfig.Memory,
			//TODO Reservation: ...,
		},
		Network: &windowsoci.WindowsNetwork{
		//TODO Bandwidth: ...,
		},
		Storage: &windowsoci.WindowsStorage{
			Bps:  &c.HostConfig.IOMaximumBandwidth,
			Iops: &c.HostConfig.IOMaximumIOps,
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

// mergeUlimits merge the Ulimits from HostConfig with daemon defaults, and update HostConfig
// It will do nothing on non-Linux platform
func (daemon *Daemon) mergeUlimits(c *containertypes.HostConfig) {
	return
}
