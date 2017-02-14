package daemon

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/container"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/restartmanager"
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

// StateChanged updates daemon state changes from containerd
func (daemon *Daemon) StateChanged(id string, e libcontainerd.StateInfo) error {
	c := daemon.containers.Get(id)
	if c == nil {
		return fmt.Errorf("no such container: %s", id)
	}

	switch e.State {
	case libcontainerd.StateOOM:
		// StateOOM is Linux specific and should never be hit on Windows
		if runtime.GOOS == "windows" {
			return errors.New("Received StateOOM from libcontainerd on Windows. This should never happen.")
		}
		daemon.updateHealthMonitor(c)
		daemon.LogContainerEvent(c, "oom")
	case libcontainerd.StateExit:

		c.Lock()
		c.StreamConfig.Wait()
		c.Reset(false)

		restart, wait, err := c.RestartManager().ShouldRestart(e.ExitCode, c.HasBeenManuallyStopped, time.Since(c.StartedAt))
		if err == nil && restart {
			c.RestartCount++
			c.SetRestarting(platformConstructExitStatus(e))
		} else {
			c.SetStopped(platformConstructExitStatus(e))
			defer daemon.autoRemove(c)
		}

		// cancel healthcheck here, they will be automatically
		// restarted if/when the container is started again
		daemon.stopHealthchecks(c)
		attributes := map[string]string{
			"exitCode": strconv.Itoa(int(e.ExitCode)),
		}
		daemon.LogContainerEventWithAttributes(c, "die", attributes)
		daemon.Cleanup(c)

		if err == nil && restart {
			go func() {
				err := <-wait
				if err == nil {
					if err = daemon.containerStart(c, "", "", false); err != nil {
						logrus.Debugf("failed to restart container: %+v", err)
					}
				}
				if err != nil {
					c.SetStopped(platformConstructExitStatus(e))
					defer daemon.autoRemove(c)
					if err != restartmanager.ErrRestartCanceled {
						logrus.Errorf("restartmanger wait error: %+v", err)
					}
				}
			}()
		}

		daemon.setStateCounter(c)

		defer c.Unlock()
		if err := c.ToDisk(); err != nil {
			return err
		}
		return daemon.postRunProcessing(c, e)
	case libcontainerd.StateExitProcess:
		if execConfig := c.ExecCommands.Get(e.ProcessID); execConfig != nil {
			ec := int(e.ExitCode)
			execConfig.Lock()
			defer execConfig.Unlock()
			execConfig.ExitCode = &ec
			execConfig.Running = false
			execConfig.StreamConfig.Wait()
			if err := execConfig.CloseStreams(); err != nil {
				logrus.Errorf("failed to cleanup exec %s streams: %s", c.ID, err)
			}

			// remove the exec command from the container's store only and not the
			// daemon's store so that the exec command can be inspected.
			c.ExecCommands.Delete(execConfig.ID)
		} else {
			logrus.Warnf("Ignoring StateExitProcess for %v but no exec command found", e)
		}
	case libcontainerd.StateStart, libcontainerd.StateRestore:
		// Container is already locked in this case
		c.SetRunning(int(e.Pid), e.State == libcontainerd.StateStart)
		c.HasBeenManuallyStopped = false
		c.HasBeenStartedBefore = true
		daemon.setStateCounter(c)

		if err := c.ToDisk(); err != nil {
			c.Reset(false)
			return err
		}
		daemon.initHealthMonitor(c)

		daemon.LogContainerEvent(c, "start")
	case libcontainerd.StatePause:
		// Container is already locked in this case
		c.Paused = true
		daemon.setStateCounter(c)
		if err := c.ToDisk(); err != nil {
			return err
		}
		daemon.updateHealthMonitor(c)
		daemon.LogContainerEvent(c, "pause")
	case libcontainerd.StateResume:
		// Container is already locked in this case
		c.Paused = false
		daemon.setStateCounter(c)
		if err := c.ToDisk(); err != nil {
			return err
		}
		daemon.updateHealthMonitor(c)
		daemon.LogContainerEvent(c, "unpause")
	}
	return nil
}

func (daemon *Daemon) autoRemove(c *container.Container) {
	c.Lock()
	ar := c.HostConfig.AutoRemove
	c.Unlock()
	if !ar {
		return
	}

	var err error
	if err = daemon.ContainerRm(c.ID, &types.ContainerRmConfig{ForceRemove: true, RemoveVolume: true}); err == nil {
		return
	}
	if c := daemon.containers.Get(c.ID); c == nil {
		return
	}

	if err != nil {
		logrus.WithError(err).WithField("container", c.ID).Error("error removing container")
	}
}
