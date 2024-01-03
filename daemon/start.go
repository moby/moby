package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"runtime"
	"time"

	"github.com/containerd/log"
	"github.com/docker/docker/api/types/backend"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libcontainerd"
	"github.com/pkg/errors"
)

// validateState verifies if the container is in a non-conflicting state.
func validateState(ctr *container.Container) error {
	ctr.Lock()
	defer ctr.Unlock()

	// Intentionally checking paused first, because a container can be
	// BOTH running AND paused. To start a paused (but running) container,
	// it must be thawed ("un-paused").
	if ctr.Paused {
		return errdefs.Conflict(errors.New("cannot start a paused container, try unpause instead"))
	} else if ctr.Running {
		// This is not an actual error, but produces a 304 "not modified"
		// when returned through the API to indicates the container is
		// already in the desired state. It's implemented as an error
		// to make the code calling this function terminate early (as
		// no further processing is needed).
		return errdefs.NotModified(errors.New("container is already running"))
	}
	if ctr.RemovalInProgress || ctr.Dead {
		return errdefs.Conflict(errors.New("container is marked for removal and cannot be started"))
	}
	return nil
}

// ContainerStart starts a container.
func (daemon *Daemon) ContainerStart(ctx context.Context, name string, hostConfig *containertypes.HostConfig, checkpoint string, checkpointDir string) error {
	daemonCfg := daemon.config()
	if checkpoint != "" && !daemonCfg.Experimental {
		return errdefs.InvalidParameter(errors.New("checkpoint is only supported in experimental mode"))
	}

	ctr, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}
	if err := validateState(ctr); err != nil {
		return err
	}

	// Windows does not have the backwards compatibility issue here.
	if runtime.GOOS != "windows" {
		// This is kept for backward compatibility - hostconfig should be passed when
		// creating a container, not during start.
		if hostConfig != nil {
			log.G(ctx).Warn("DEPRECATED: Setting host configuration options when the container starts is deprecated and has been removed in Docker 1.12")
			oldNetworkMode := ctr.HostConfig.NetworkMode
			if err := daemon.setSecurityOptions(&daemonCfg.Config, ctr, hostConfig); err != nil {
				return errdefs.InvalidParameter(err)
			}
			if err := daemon.mergeAndVerifyLogConfig(&hostConfig.LogConfig); err != nil {
				return errdefs.InvalidParameter(err)
			}
			if err := daemon.setHostConfig(ctr, hostConfig); err != nil {
				return errdefs.InvalidParameter(err)
			}
			newNetworkMode := ctr.HostConfig.NetworkMode
			if string(oldNetworkMode) != string(newNetworkMode) {
				// if user has change the network mode on starting, clean up the
				// old networks. It is a deprecated feature and has been removed in Docker 1.12
				ctr.NetworkSettings.Networks = nil
			}
			if err := ctr.CheckpointTo(daemon.containersReplica); err != nil {
				return errdefs.System(err)
			}
			ctr.InitDNSHostConfig()
		}
	} else {
		if hostConfig != nil {
			return errdefs.InvalidParameter(errors.New("Supplying a hostconfig on start is not supported. It should be supplied on create"))
		}
	}

	// check if hostConfig is in line with the current system settings.
	// It may happen cgroups are umounted or the like.
	if _, err = daemon.verifyContainerSettings(daemonCfg, ctr.HostConfig, nil, false); err != nil {
		return errdefs.InvalidParameter(err)
	}
	// Adapt for old containers in case we have updates in this function and
	// old containers never have chance to call the new function in create stage.
	if hostConfig != nil {
		if err := daemon.adaptContainerSettings(&daemonCfg.Config, ctr.HostConfig, false); err != nil {
			return errdefs.InvalidParameter(err)
		}
	}
	return daemon.containerStart(ctx, daemonCfg, ctr, checkpoint, checkpointDir, true)
}

// containerStart prepares the container to run by setting up everything the
// container needs, such as storage and networking, as well as links
// between containers. The container is left waiting for a signal to
// begin running.
func (daemon *Daemon) containerStart(ctx context.Context, daemonCfg *configStore, container *container.Container, checkpoint string, checkpointDir string, resetRestartManager bool) (retErr error) {
	start := time.Now()
	container.Lock()
	defer container.Unlock()

	if resetRestartManager && container.Running { // skip this check if already in restarting step and resetRestartManager==false
		return nil
	}

	if container.RemovalInProgress || container.Dead {
		return errdefs.Conflict(errors.New("container is marked for removal and cannot be started"))
	}

	if checkpointDir != "" {
		// TODO(mlaventure): how would we support that?
		return errdefs.Forbidden(errors.New("custom checkpointdir is not supported"))
	}

	// if we encounter an error during start we need to ensure that any other
	// setup has been cleaned up properly
	defer func() {
		if retErr != nil {
			container.SetError(retErr)
			// if no one else has set it, make sure we don't leave it at zero
			if container.ExitCode() == 0 {
				container.SetExitCode(exitUnknown)
			}
			if err := container.CheckpointTo(daemon.containersReplica); err != nil {
				log.G(ctx).Errorf("%s: failed saving state on start failure: %v", container.ID, err)
			}
			container.Reset(false)

			daemon.Cleanup(container)
			// if containers AutoRemove flag is set, remove it after clean up
			if container.HostConfig.AutoRemove {
				container.Unlock()
				if err := daemon.containerRm(&daemonCfg.Config, container.ID, &backend.ContainerRmConfig{ForceRemove: true, RemoveVolume: true}); err != nil {
					log.G(ctx).Errorf("can't remove container %s: %v", container.ID, err)
				}
				container.Lock()
			}
		}
	}()

	if err := daemon.conditionalMountOnStart(container); err != nil {
		return err
	}

	if err := daemon.initializeNetworking(&daemonCfg.Config, container); err != nil {
		return err
	}

	spec, err := daemon.createSpec(ctx, daemonCfg, container)
	if err != nil {
		// Any error that occurs while creating the spec, even if it's the
		// result of an invalid container config, must be considered a System
		// error (internal server error), as it's not an error with the request
		// to start the container.
		//
		// Invalid configuration in the config itself must be validated when
		// creating the container (creating its config), but some errors are
		// dependent on the current state, for example when starting a container
		// that shares a namespace with another container, and that container
		// is not running (or missing).
		return errdefs.System(err)
	}

	if resetRestartManager {
		container.ResetRestartManager(true)
		container.HasBeenManuallyStopped = false
	}

	if err := daemon.saveAppArmorConfig(container); err != nil {
		return err
	}

	if checkpoint != "" {
		checkpointDir, err = getCheckpointDir(checkpointDir, checkpoint, container.Name, container.ID, container.CheckpointDir(), false)
		if err != nil {
			return err
		}
	}

	shim, createOptions, err := daemon.getLibcontainerdCreateOptions(daemonCfg, container)
	if err != nil {
		return err
	}

	ctr, err := libcontainerd.ReplaceContainer(ctx, daemon.containerd, container.ID, spec, shim, createOptions)
	if err != nil {
		return setExitCodeFromError(container.SetExitCode, err)
	}

	// TODO(mlaventure): we need to specify checkpoint options here
	tsk, err := ctr.Start(context.TODO(), // Passing ctx to ctr.Start caused integration tests to be stuck in the cleanup phase
		checkpointDir, container.StreamConfig.Stdin() != nil || container.Config.Tty,
		container.InitializeStdio)
	if err != nil {
		if err := ctr.Delete(context.Background()); err != nil {
			log.G(ctx).WithError(err).WithField("container", container.ID).
				Error("failed to delete failed start container")
		}
		return setExitCodeFromError(container.SetExitCode, err)
	}

	container.HasBeenManuallyRestarted = false
	container.SetRunning(ctr, tsk, true)
	container.HasBeenStartedBefore = true
	daemon.setStateCounter(container)

	daemon.initHealthMonitor(container)

	if err := container.CheckpointTo(daemon.containersReplica); err != nil {
		log.G(ctx).WithError(err).WithField("container", container.ID).
			Errorf("failed to store container")
	}

	daemon.LogContainerEvent(container, events.ActionStart)
	containerActions.WithValues("start").UpdateSince(start)

	return nil
}

// Cleanup releases any network resources allocated to the container along with any rules
// around how containers are linked together.  It also unmounts the container's root filesystem.
func (daemon *Daemon) Cleanup(container *container.Container) {
	// Microsoft HCS containers get in a bad state if host resources are
	// released while the container still exists.
	if ctr, ok := container.C8dContainer(); ok {
		if err := ctr.Delete(context.Background()); err != nil {
			log.G(context.TODO()).Errorf("%s cleanup: failed to delete container from containerd: %v", container.ID, err)
		}
	}

	daemon.releaseNetwork(container)

	if err := container.UnmountIpcMount(); err != nil {
		log.G(context.TODO()).Warnf("%s cleanup: failed to unmount IPC: %s", container.ID, err)
	}

	if err := daemon.conditionalUnmountOnCleanup(container); err != nil {
		// FIXME: remove once reference counting for graphdrivers has been refactored
		// Ensure that all the mounts are gone
		if mountid, err := daemon.imageService.GetLayerMountID(container.ID); err == nil {
			daemon.cleanupMountsByID(mountid)
		}
	}

	if err := container.UnmountSecrets(); err != nil {
		log.G(context.TODO()).Warnf("%s cleanup: failed to unmount secrets: %s", container.ID, err)
	}

	if err := recursiveUnmount(container.Root); err != nil {
		log.G(context.TODO()).WithError(err).WithField("container", container.ID).Warn("Error while cleaning up container resource mounts.")
	}

	for _, eConfig := range container.ExecCommands.Commands() {
		daemon.unregisterExecCommand(container, eConfig)
	}

	if container.BaseFS != "" {
		if err := container.UnmountVolumes(daemon.LogVolumeEvent); err != nil {
			log.G(context.TODO()).Warnf("%s cleanup: Failed to umount volumes: %v", container.ID, err)
		}
	}

	container.CancelAttachContext()
}
