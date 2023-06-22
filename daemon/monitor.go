package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"strconv"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/errdefs"
	libcontainerdtypes "github.com/docker/docker/libcontainerd/types"
	"github.com/docker/docker/restartmanager"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (daemon *Daemon) setStateCounter(c *container.Container) {
	switch c.StateString() {
	case "paused":
		stateCtr.set(c.ID, "paused")
	case "running":
		stateCtr.set(c.ID, "running")
	default:
		stateCtr.set(c.ID, "stopped")
	}
}

func (daemon *Daemon) handleContainerExit(c *container.Container, e *libcontainerdtypes.EventInfo) error {
	var exitStatus container.ExitStatus
	c.Lock()

	cfg := daemon.config()

	// Health checks will be automatically restarted if/when the
	// container is started again.
	daemon.stopHealthchecks(c)

	tsk, ok := c.Task()
	if ok {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		es, err := tsk.Delete(ctx)
		cancel()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				logrus.ErrorKey: err,
				"container":     c.ID,
			}).Warn("failed to delete container from containerd")
		} else {
			exitStatus = container.ExitStatus{
				ExitCode: int(es.ExitCode()),
				ExitedAt: es.ExitTime(),
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	c.StreamConfig.Wait(ctx)
	cancel()

	c.Reset(false)

	if e != nil {
		exitStatus.ExitCode = int(e.ExitCode)
		exitStatus.ExitedAt = e.ExitedAt
		if e.Error != nil {
			c.SetError(e.Error)
		}
	}

	daemonShutdown := daemon.IsShuttingDown()
	execDuration := time.Since(c.StartedAt)
	restart, wait, err := c.RestartManager().ShouldRestart(uint32(exitStatus.ExitCode), daemonShutdown || c.HasBeenManuallyStopped, execDuration)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			logrus.ErrorKey:          err,
			"container":              c.ID,
			"restartCount":           c.RestartCount,
			"exitStatus":             exitStatus,
			"daemonShuttingDown":     daemonShutdown,
			"hasBeenManuallyStopped": c.HasBeenManuallyStopped,
			"execDuration":           execDuration,
		}).Warn("ShouldRestart failed, container will not be restarted")
		restart = false
	}

	attributes := map[string]string{
		"exitCode":     strconv.Itoa(exitStatus.ExitCode),
		"execDuration": strconv.Itoa(int(execDuration.Seconds())),
	}
	daemon.Cleanup(c)

	if restart {
		c.RestartCount++
		logrus.WithFields(logrus.Fields{
			"container":     c.ID,
			"restartCount":  c.RestartCount,
			"exitStatus":    exitStatus,
			"manualRestart": c.HasBeenManuallyRestarted,
		}).Debug("Restarting container")
		c.SetRestarting(&exitStatus)
	} else {
		c.SetStopped(&exitStatus)
		if !c.HasBeenManuallyRestarted {
			defer daemon.autoRemove(&cfg.Config, c)
		}
	}
	defer c.Unlock() // needs to be called before autoRemove

	daemon.setStateCounter(c)
	cpErr := c.CheckpointTo(daemon.containersReplica)

	daemon.LogContainerEventWithAttributes(c, "die", attributes)

	if restart {
		go func() {
			err := <-wait
			if err == nil {
				// daemon.netController is initialized when daemon is restoring containers.
				// But containerStart will use daemon.netController segment.
				// So to avoid panic at startup process, here must wait util daemon restore done.
				daemon.waitForStartupDone()
				cfg := daemon.config() // Apply the most up-to-date daemon config to the restarted container.
				if err = daemon.containerStart(context.Background(), cfg, c, "", "", false); err != nil {
					logrus.Debugf("failed to restart container: %+v", err)
				}
			}
			if err != nil {
				c.Lock()
				c.SetStopped(&exitStatus)
				daemon.setStateCounter(c)
				c.CheckpointTo(daemon.containersReplica)
				c.Unlock()
				defer daemon.autoRemove(&cfg.Config, c)
				if err != restartmanager.ErrRestartCanceled {
					logrus.Errorf("restartmanger wait error: %+v", err)
				}
			}
		}()
	}

	return cpErr
}

// ProcessEvent is called by libcontainerd whenever an event occurs
func (daemon *Daemon) ProcessEvent(id string, e libcontainerdtypes.EventType, ei libcontainerdtypes.EventInfo) error {
	c, err := daemon.GetContainer(id)
	if err != nil {
		return errors.Wrapf(err, "could not find container %s", id)
	}

	switch e {
	case libcontainerdtypes.EventOOM:
		// StateOOM is Linux specific and should never be hit on Windows
		if isWindows {
			return errors.New("received StateOOM from libcontainerd on Windows. This should never happen")
		}

		c.Lock()
		defer c.Unlock()
		c.OOMKilled = true
		daemon.updateHealthMonitor(c)
		if err := c.CheckpointTo(daemon.containersReplica); err != nil {
			return err
		}

		daemon.LogContainerEvent(c, "oom")
	case libcontainerdtypes.EventExit:
		if ei.ProcessID == ei.ContainerID {
			return daemon.handleContainerExit(c, &ei)
		}

		exitCode := 127
		if execConfig := c.ExecCommands.Get(ei.ProcessID); execConfig != nil {
			ec := int(ei.ExitCode)
			execConfig.Lock()
			defer execConfig.Unlock()

			// Remove the exec command from the container's store only and not the
			// daemon's store so that the exec command can be inspected. Remove it
			// before mutating execConfig to maintain the invariant that
			// c.ExecCommands only contains execs that have not exited.
			c.ExecCommands.Delete(execConfig.ID)

			execConfig.ExitCode = &ec
			execConfig.Running = false

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			execConfig.StreamConfig.Wait(ctx)
			cancel()

			if err := execConfig.CloseStreams(); err != nil {
				logrus.Errorf("failed to cleanup exec %s streams: %s", c.ID, err)
			}

			exitCode = ec

			// If the exec failed at start in such a way that containerd
			// publishes an exit event for it, we will race processing the event
			// with daemon.ContainerExecStart() removing the exec from
			// c.ExecCommands. If we win the race, we will find that there is no
			// process to clean up. (And ContainerExecStart will clobber the
			// exit code we set.) Prevent a nil-dereferenc panic in that
			// situation to restore the status quo where this is merely a
			// logical race condition.
			if execConfig.Process != nil {
				go func() {
					if _, err := execConfig.Process.Delete(context.Background()); err != nil {
						logrus.WithFields(logrus.Fields{
							logrus.ErrorKey: err,
							"container":     ei.ContainerID,
							"process":       ei.ProcessID,
						}).Warn("failed to delete process")
					}
				}()
			}
		}
		attributes := map[string]string{
			"execID":   ei.ProcessID,
			"exitCode": strconv.Itoa(exitCode),
		}
		daemon.LogContainerEventWithAttributes(c, "exec_die", attributes)
	case libcontainerdtypes.EventStart:
		c.Lock()
		defer c.Unlock()

		// This is here to handle start not generated by docker
		if !c.Running {
			ctr, err := daemon.containerd.LoadContainer(context.Background(), c.ID)
			if err != nil {
				if errdefs.IsNotFound(err) {
					// The container was started by not-docker and so could have been deleted by
					// not-docker before we got around to loading it from containerd.
					logrus.WithFields(logrus.Fields{
						logrus.ErrorKey: err,
						"container":     c.ID,
					}).Debug("could not load containerd container for start event")
					return nil
				}
				return err
			}
			tsk, err := ctr.Task(context.Background())
			if err != nil {
				if errdefs.IsNotFound(err) {
					logrus.WithFields(logrus.Fields{
						logrus.ErrorKey: err,
						"container":     c.ID,
					}).Debug("failed to load task for externally-started container")
					return nil
				}
				return err
			}
			c.SetRunning(ctr, tsk, false)
			c.HasBeenManuallyStopped = false
			c.HasBeenStartedBefore = true
			daemon.setStateCounter(c)

			daemon.initHealthMonitor(c)

			if err := c.CheckpointTo(daemon.containersReplica); err != nil {
				return err
			}
			daemon.LogContainerEvent(c, "start")
		}

	case libcontainerdtypes.EventPaused:
		c.Lock()
		defer c.Unlock()

		if !c.Paused {
			c.Paused = true
			daemon.setStateCounter(c)
			daemon.updateHealthMonitor(c)
			if err := c.CheckpointTo(daemon.containersReplica); err != nil {
				return err
			}
			daemon.LogContainerEvent(c, "pause")
		}
	case libcontainerdtypes.EventResumed:
		c.Lock()
		defer c.Unlock()

		if c.Paused {
			c.Paused = false
			daemon.setStateCounter(c)
			daemon.updateHealthMonitor(c)

			if err := c.CheckpointTo(daemon.containersReplica); err != nil {
				return err
			}
			daemon.LogContainerEvent(c, "unpause")
		}
	}
	return nil
}

func (daemon *Daemon) autoRemove(cfg *config.Config, c *container.Container) {
	c.Lock()
	ar := c.HostConfig.AutoRemove
	c.Unlock()
	if !ar {
		return
	}

	err := daemon.containerRm(cfg, c.ID, &types.ContainerRmConfig{ForceRemove: true, RemoveVolume: true})
	if err == nil {
		return
	}
	if c := daemon.containers.Get(c.ID); c == nil {
		return
	}

	logrus.WithFields(logrus.Fields{logrus.ErrorKey: err, "container": c.ID}).Error("error removing container")
}
