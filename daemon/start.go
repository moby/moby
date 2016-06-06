package daemon

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"syscall"

	"google.golang.org/grpc"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/container"
	"github.com/docker/docker/errors"
	"github.com/docker/docker/libcontainerd"
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
		return fmt.Errorf("Cannot start a paused container, try unpause instead.")
	}

	if container.IsRunning() {
		err := fmt.Errorf("Container already started")
		return errors.NewErrorWithStatusCode(err, http.StatusNotModified)
	}

	// Windows does not have the backwards compatibility issue here.
	if runtime.GOOS != "windows" {
		// This is kept for backward compatibility - hostconfig should be passed when
		// creating a container, not during start.
		if hostConfig != nil {
			logrus.Warn("DEPRECATED: Setting host configuration options when the container starts is deprecated and will be removed in Docker 1.12")
			oldNetworkMode := container.HostConfig.NetworkMode
			if err := daemon.setSecurityOptions(container, hostConfig); err != nil {
				return err
			}
			if err := daemon.mergeAndVerifyLogConfig(&hostConfig.LogConfig); err != nil {
				return err
			}
			if err := daemon.setHostConfig(container, hostConfig); err != nil {
				return err
			}
			newNetworkMode := container.HostConfig.NetworkMode
			if string(oldNetworkMode) != string(newNetworkMode) {
				// if user has change the network mode on starting, clean up the
				// old networks. It is a deprecated feature and will be removed in Docker 1.12
				container.NetworkSettings.Networks = nil
				if err := container.ToDisk(); err != nil {
					return err
				}
			}
			container.InitDNSHostConfig()
		}
	} else {
		if hostConfig != nil {
			return fmt.Errorf("Supplying a hostconfig on start is not supported. It should be supplied on create")
		}
	}

	// check if hostConfig is in line with the current system settings.
	// It may happen cgroups are umounted or the like.
	if _, err = daemon.verifyContainerSettings(container.HostConfig, nil, false); err != nil {
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
		return fmt.Errorf("Container is marked for removal and cannot be started.")
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

	spec, err := daemon.createSpec(container)
	if err != nil {
		return err
	}

	if err := daemon.containerd.Create(container.ID, *spec, libcontainerd.WithRestartManager(container.RestartManager(true))); err != nil {
		errDesc := grpc.ErrorDesc(err)
		logrus.Errorf("Create container failed with error: %s", errDesc)
		// if we receive an internal error from the initial start of a container then lets
		// return it instead of entering the restart loop
		// set to 127 for container cmd not found/does not exist)
		if strings.Contains(errDesc, "executable file not found") ||
			strings.Contains(errDesc, "no such file or directory") ||
			strings.Contains(errDesc, "system cannot find the file specified") {
			container.ExitCode = 127
		}
		// set to 126 for container cmd can't be invoked errors
		if strings.Contains(errDesc, syscall.EACCES.Error()) {
			container.ExitCode = 126
		}

		container.Reset(false)

		return fmt.Errorf("%s", errDesc)
	}

	return nil
}

// Cleanup releases any network resources allocated to the container along with any rules
// around how containers are linked together.  It also unmounts the container's root filesystem.
func (daemon *Daemon) Cleanup(container *container.Container) {
	daemon.releaseNetwork(container)

	container.UnmountIpcMounts(detachMounted)

	if err := daemon.conditionalUnmountOnCleanup(container); err != nil {
		// FIXME: remove once reference counting for graphdrivers has been refactored
		// Ensure that all the mounts are gone
		if mountid, err := daemon.layerStore.GetMountID(container.ID); err == nil {
			daemon.cleanupMountsByID(mountid)
		}
	}

	for _, eConfig := range container.ExecCommands.Commands() {
		daemon.unregisterExecCommand(container, eConfig)
	}

	if container.BaseFS != "" {
		if err := container.UnmountVolumes(false, daemon.LogVolumeEvent); err != nil {
			logrus.Warnf("%s cleanup: Failed to umount volumes: %v", container.ID, err)
		}
	}
	container.CancelAttachContext()
}
