package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/log"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/container"
	mobyc8dstore "github.com/docker/docker/daemon/containerd"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libcontainerd"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
func (daemon *Daemon) ContainerStart(ctx context.Context, name string, checkpoint string, checkpointDir string) error {
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

	// check if hostConfig is in line with the current system settings.
	// It may happen cgroups are unmounted or the like.
	if _, err = daemon.verifyContainerSettings(daemonCfg, ctr.HostConfig, nil, false); err != nil {
		return errdefs.InvalidParameter(err)
	}

	return daemon.containerStart(ctx, daemonCfg, ctr, checkpoint, checkpointDir, true)
}

// containerStart prepares the container to run by setting up everything the
// container needs, such as storage and networking, as well as links
// between containers. The container is left waiting for a signal to
// begin running.
func (daemon *Daemon) containerStart(ctx context.Context, daemonCfg *configStore, ctr *container.Container, checkpoint string, checkpointDir string, resetRestartManager bool) (retErr error) {
	ctx, span := otel.Tracer("").Start(ctx, "daemon.containerStart", trace.WithAttributes(
		attribute.String("container.ID", ctr.ID),
		attribute.String("container.Name", ctr.Name)))
	defer span.End()

	start := time.Now()
	ctr.Lock()
	defer ctr.Unlock()

	if resetRestartManager && ctr.Running { // skip this check if already in restarting step and resetRestartManager==false
		return nil
	}

	if ctr.RemovalInProgress || ctr.Dead {
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
			ctr.SetError(retErr)
			// if no one else has set it, make sure we don't leave it at zero
			if ctr.ExitCode() == 0 {
				ctr.SetExitCode(exitUnknown)
			}
			if err := ctr.CheckpointTo(context.WithoutCancel(ctx), daemon.containersReplica); err != nil {
				log.G(ctx).Errorf("%s: failed saving state on start failure: %v", ctr.ID, err)
			}
			ctr.Reset(false)

			daemon.Cleanup(context.WithoutCancel(ctx), ctr)
			// if containers AutoRemove flag is set, remove it after clean up
			if ctr.HostConfig.AutoRemove {
				ctr.Unlock()
				if err := daemon.containerRm(&daemonCfg.Config, ctr.ID, &backend.ContainerRmConfig{ForceRemove: true, RemoveVolume: true}); err != nil {
					log.G(ctx).Errorf("can't remove container %s: %v", ctr.ID, err)
				}
				ctr.Lock()
			}
		}
	}()

	if err := daemon.conditionalMountOnStart(ctr); err != nil {
		return err
	}

	if err := daemon.initializeNetworking(ctx, &daemonCfg.Config, ctr); err != nil {
		return err
	}

	mnts, err := daemon.setupContainerDirs(ctr)
	if err != nil {
		return err
	}

	m, cleanup, err := daemon.setupMounts(ctx, ctr)
	if err != nil {
		return err
	}
	mnts = append(mnts, m...)
	defer cleanup(context.WithoutCancel(ctx))

	spec, err := daemon.createSpec(ctx, daemonCfg, ctr, mnts)
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
		ctr.ResetRestartManager(true)
		ctr.HasBeenManuallyStopped = false
	}

	if err := daemon.saveAppArmorConfig(ctr); err != nil {
		return err
	}

	if checkpoint != "" {
		checkpointDir, err = getCheckpointDir(checkpointDir, checkpoint, ctr.Name, ctr.ID, ctr.CheckpointDir(), false)
		if err != nil {
			return err
		}
	}

	shim, createOptions, err := daemon.getLibcontainerdCreateOptions(daemonCfg, ctr)
	if err != nil {
		return err
	}

	c8dCtr, err := libcontainerd.ReplaceContainer(ctx, daemon.containerd, ctr.ID, spec, shim, createOptions, func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		// Only set the image if we are using containerd for image storage.
		// This is for metadata purposes only.
		// Other lower-level components may make use of this information.
		is, ok := daemon.imageService.(*mobyc8dstore.ImageService)
		if !ok {
			return nil
		}
		img, err := is.ResolveImage(ctx, ctr.Config.Image)
		if err != nil {
			log.G(ctx).WithError(err).WithField("container", ctr.ID).Warn("Failed to resolve containerd image reference")
			return nil
		}
		c.Image = img.Name
		return nil
	})
	if err != nil {
		return setExitCodeFromError(ctr.SetExitCode, err)
	}
	defer func() {
		if retErr != nil {
			if err := c8dCtr.Delete(context.WithoutCancel(ctx)); err != nil {
				log.G(ctx).WithError(err).WithField("container", ctr.ID).
					Error("failed to delete failed start container")
			}
		}
	}()

	startupTime := time.Now()
	// TODO(mlaventure): we need to specify checkpoint options here
	tsk, err := c8dCtr.NewTask(context.WithoutCancel(ctx), // passing a cancelable ctx caused integration tests to be stuck in the cleanup phase
		checkpointDir, ctr.StreamConfig.Stdin() != nil || ctr.Config.Tty,
		ctr.InitializeStdio)
	if err != nil {
		return setExitCodeFromError(ctr.SetExitCode, err)
	}
	defer func() {
		if retErr != nil {
			if err := tsk.ForceDelete(context.WithoutCancel(ctx)); err != nil {
				log.G(ctx).WithError(err).WithField("container", ctr.ID).
					Error("failed to delete task after fail start")
			}
		}
	}()

	if err := daemon.initializeCreatedTask(ctx, tsk, ctr, spec); err != nil {
		return err
	}

	if err := tsk.Start(context.WithoutCancel(ctx)); err != nil { // passing a cancelable ctx caused integration tests to be stuck in the cleanup phase
		return setExitCodeFromError(ctr.SetExitCode, err)
	}

	ctr.HasBeenManuallyRestarted = false
	ctr.SetRunning(c8dCtr, tsk, startupTime)
	ctr.HasBeenStartedBefore = true
	daemon.setStateCounter(ctr)

	daemon.initHealthMonitor(ctr)

	if err := ctr.CheckpointTo(context.WithoutCancel(ctx), daemon.containersReplica); err != nil {
		log.G(ctx).WithError(err).WithField("container", ctr.ID).
			Errorf("failed to store container")
	}

	daemon.LogContainerEvent(ctr, events.ActionStart)
	containerActions.WithValues("start").UpdateSince(start)

	return nil
}

// Cleanup releases any network resources allocated to the container along with any rules
// around how containers are linked together.  It also unmounts the container's root filesystem.
func (daemon *Daemon) Cleanup(ctx context.Context, ctr *container.Container) {
	// Microsoft HCS containers get in a bad state if host resources are
	// released while the container still exists.
	if c8dCtr, ok := ctr.C8dContainer(); ok {
		if err := c8dCtr.Delete(context.Background()); err != nil {
			log.G(ctx).Errorf("%s cleanup: failed to delete container from containerd: %v", ctr.ID, err)
		}
	}

	daemon.releaseNetwork(ctx, ctr)

	if err := ctr.UnmountIpcMount(); err != nil {
		log.G(ctx).Warnf("%s cleanup: failed to unmount IPC: %s", ctr.ID, err)
	}

	if err := daemon.conditionalUnmountOnCleanup(ctr); err != nil {
		// FIXME: remove once reference counting for graphdrivers has been refactored
		// Ensure that all the mounts are gone
		if mountid, err := daemon.imageService.GetLayerMountID(ctr.ID); err == nil {
			daemon.cleanupMountsByID(mountid)
		}
	}

	if err := ctr.UnmountSecrets(); err != nil {
		log.G(ctx).Warnf("%s cleanup: failed to unmount secrets: %s", ctr.ID, err)
	}

	if err := recursiveUnmount(ctr.Root); err != nil {
		log.G(ctx).WithError(err).WithField("container", ctr.ID).Warn("Error while cleaning up container resource mounts.")
	}

	for _, eConfig := range ctr.ExecCommands.Commands() {
		daemon.unregisterExecCommand(ctr, eConfig)
	}

	if ctr.BaseFS != "" {
		if err := ctr.UnmountVolumes(ctx, daemon.LogVolumeEvent); err != nil {
			log.G(ctx).Warnf("%s cleanup: Failed to umount volumes: %v", ctr.ID, err)
		}
	}

	ctr.CancelAttachContext()
}
