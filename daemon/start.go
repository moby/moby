package daemon

import (
	"runtime"

	"github.com/Sirupsen/logrus"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/pkg/promise"
	"github.com/docker/docker/runconfig"
)

// ContainerStart starts a container.
func (daemon *Daemon) ContainerStart(name string, hostConfig *runconfig.HostConfig) error {
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	if container.isPaused() {
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
			container.Lock()
			if err := parseSecurityOpt(container, hostConfig); err != nil {
				container.Unlock()
				return err
			}
			container.Unlock()
			if err := daemon.setHostConfig(container, hostConfig); err != nil {
				return err
			}
			initDNSHostConfig(container)
		}
	} else {
		if hostConfig != nil {
			return derr.ErrorCodeHostConfigStart
		}
	}

	// check if hostConfig is in line with the current system settings.
	// It may happen cgroups are umounted or the like.
	if _, err = daemon.verifyContainerSettings(container.hostConfig, nil); err != nil {
		return err
	}

	if err := daemon.containerStart(container); err != nil {
		return err
	}

	return nil
}

// Start starts a container
func (daemon *Daemon) Start(container *Container) error {
	return daemon.containerStart(container)
}

// containerStart prepares the container to run by setting up everything the
// container needs, such as storage and networking, as well as links
// between containers. The container is left waiting for a signal to
// begin running.
func (daemon *Daemon) containerStart(container *Container) (err error) {
	container.Lock()
	defer container.Unlock()

	if container.Running {
		return nil
	}

	if container.removalInProgress || container.Dead {
		return derr.ErrorCodeContainerBeingRemoved
	}

	// if we encounter an error during start we need to ensure that any other
	// setup has been cleaned up properly
	defer func() {
		if err != nil {
			container.setError(err)
			// if no one else has set it, make sure we don't leave it at zero
			if container.ExitCode == 0 {
				container.ExitCode = 128
			}
			container.toDisk()
			daemon.Cleanup(container)
			daemon.LogContainerEvent(container, "die")
		}
	}()

	if err := daemon.conditionalMountOnStart(container); err != nil {
		return err
	}

	// Make sure NetworkMode has an acceptable value. We do this to ensure
	// backwards API compatibility.
	container.hostConfig = runconfig.SetDefaultNetModeIfBlank(container.hostConfig)

	if err := daemon.initializeNetworking(container); err != nil {
		return err
	}
	linkedEnv, err := daemon.setupLinkedContainers(container)
	if err != nil {
		return err
	}
	if err := container.setupWorkingDirectory(); err != nil {
		return err
	}
	env := container.createDaemonEnvironment(linkedEnv)
	if err := daemon.populateCommand(container, env); err != nil {
		return err
	}

	if !container.hostConfig.IpcMode.IsContainer() && !container.hostConfig.IpcMode.IsHost() {
		if err := daemon.setupIpcDirs(container); err != nil {
			return err
		}
	}

	mounts, err := daemon.setupMounts(container)
	if err != nil {
		return err
	}
	mounts = append(mounts, container.ipcMounts()...)

	container.command.Mounts = mounts
	if err := daemon.waitForStart(container); err != nil {
		return err
	}
	container.HasBeenStartedBefore = true
	return nil
}

func (daemon *Daemon) waitForStart(container *Container) error {
	container.monitor = daemon.newContainerMonitor(container, container.hostConfig.RestartPolicy)

	// block until we either receive an error from the initial start of the container's
	// process or until the process is running in the container
	select {
	case <-container.monitor.startSignal:
	case err := <-promise.Go(container.monitor.Start):
		return err
	}

	return nil
}

// Cleanup releases any network resources allocated to the container along with any rules
// around how containers are linked together.  It also unmounts the container's root filesystem.
func (daemon *Daemon) Cleanup(container *Container) {
	daemon.releaseNetwork(container)

	container.unmountIpcMounts(detachMounted)

	daemon.conditionalUnmountOnCleanup(container)

	for _, eConfig := range container.execCommands.Commands() {
		daemon.unregisterExecCommand(container, eConfig)
	}

	if err := container.unmountVolumes(false); err != nil {
		logrus.Warnf("%s cleanup: Failed to umount volumes: %v", container.ID, err)
	}
}
