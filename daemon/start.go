package daemon

import (
	"runtime"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/container"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/runconfig"
	containertypes "github.com/docker/engine-api/types/container"
)

// ContainerStart starts a container.
func (daemon *Daemon) ContainerStart(name string, hostConfig *containertypes.HostConfig) error {
	container, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}

	if container.IsPaused() {
		return derr.ErrorCodeStartPaused
	}

	if container.IsRunning() {
		return derr.ErrorCodeAlreadyStarted
	}

	// Windows does not have the backwards compatibility issue here.
	if runtime.GOOS != "windows" {
		// This is kept for backward compatibility - hostconfig should be passed when
		// creating a container, not during start.
		if hostConfig != nil {
			logrus.Warn("DEPRECATED: Setting host configuration options when the container starts is deprecated and will be removed in Docker 1.12")
			if err := daemon.setSecurityOptions(container, hostConfig); err != nil {
				return err
			}
			if err := daemon.setHostConfig(container, hostConfig); err != nil {
				return err
			}
			container.InitDNSHostConfig()
		}
	} else {
		if hostConfig != nil {
			return derr.ErrorCodeHostConfigStart
		}
	}

	// check if hostConfig is in line with the current system settings.
	// It may happen cgroups are umounted or the like.
	if _, err = daemon.verifyContainerSettings(container.HostConfig, nil); err != nil {
		return err
	}
	// Adapt for old containers in case we have updates in this function and
	// old containers never have chance to call the new function in create stage.
	if err := daemon.adaptContainerSettings(container.HostConfig, false); err != nil {
		return err
	}

	return daemon.containerStart(container)
}

// Start starts a container
func (daemon *Daemon) Start(container *container.Container) error {
	return daemon.containerStart(container)
}

// containerStart prepares the container to run by setting up everything the
// container needs, such as storage and networking, as well as links
// between containers. The container is left waiting for a signal to
// begin running.
func (daemon *Daemon) containerStart(container *container.Container) (err error) {
	container.Lock()
	defer container.Unlock()

	if container.Running {
		return nil
	}

	if container.RemovalInProgress || container.Dead {
		return derr.ErrorCodeContainerBeingRemoved
	}

	// if we encounter an error during start we need to ensure that any other
	// setup has been cleaned up properly
	defer func() {
		if err != nil {
			container.SetError(err)
			// if no one else has set it, make sure we don't leave it at zero
			if container.ExitCode == 0 {
				container.ExitCode = 128
			}
			container.ToDisk()
			daemon.Cleanup(container)
			daemon.LogContainerEvent(container, "die")
		}
	}()

	if err := daemon.conditionalMountOnStart(container); err != nil {
		return err
	}

	// Make sure NetworkMode has an acceptable value. We do this to ensure
	// backwards API compatibility.
	container.HostConfig = runconfig.SetDefaultNetModeIfBlank(container.HostConfig)

	if err := daemon.initializeNetworking(container); err != nil {
		return err
	}
	linkedEnv, err := daemon.setupLinkedContainers(container)
	if err != nil {
		return err
	}
	if err := container.SetupWorkingDirectory(); err != nil {
		return err
	}
	env := container.CreateDaemonEnvironment(linkedEnv)
	if err := daemon.populateCommand(container, env); err != nil {
		return err
	}

	if !container.HostConfig.IpcMode.IsContainer() && !container.HostConfig.IpcMode.IsHost() {
		if err := daemon.setupIpcDirs(container); err != nil {
			return err
		}
	}

	mounts, err := daemon.setupMounts(container)
	if err != nil {
		return err
	}
	mounts = append(mounts, container.IpcMounts()...)
	mounts = append(mounts, container.TmpfsMounts()...)

	container.Command.Mounts = mounts
	container.Unlock()

	// don't lock waitForStart because it has potential risk of blocking
	// which will lead to dead lock, forever.
	if err := daemon.waitForStart(container); err != nil {
		container.Lock()
		return err
	}
	container.Lock()
	container.HasBeenStartedBefore = true
	return nil
}

func (daemon *Daemon) waitForStart(container *container.Container) error {
	return container.StartMonitor(daemon, container.HostConfig.RestartPolicy)
}

// Cleanup releases any network resources allocated to the container along with any rules
// around how containers are linked together.  It also unmounts the container's root filesystem.
func (daemon *Daemon) Cleanup(container *container.Container) {
	daemon.releaseNetwork(container)

	container.UnmountIpcMounts(detachMounted)

	daemon.conditionalUnmountOnCleanup(container)

	for _, eConfig := range container.ExecCommands.Commands() {
		daemon.unregisterExecCommand(container, eConfig)
	}

	if err := container.UnmountVolumes(false, daemon.LogVolumeEvent); err != nil {
		logrus.Warnf("%s cleanup: Failed to umount volumes: %v", container.ID, err)
	}
}
