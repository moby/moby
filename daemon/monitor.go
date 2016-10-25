package daemon

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon/exec"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/restartmanager"
	"github.com/docker/docker/runconfig"
)

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
		// if container's AutoRemove flag is set, remove it after clean up
		autoRemove := func() {
			if c.HostConfig.AutoRemove {
				if err := daemon.ContainerRm(c.ID, &types.ContainerRmConfig{ForceRemove: true, RemoveVolume: true}); err != nil {
					logrus.Errorf("can't remove container %s: %v", c.ID, err)
				}
			}
		}

		c.Lock()
		c.Wait()
		c.Reset(false)

		restart, wait, err := c.RestartManager().ShouldRestart(e.ExitCode, false, time.Since(c.StartedAt))
		if err == nil && restart {
			c.RestartCount++
			c.SetRestarting(platformConstructExitStatus(e))
		} else {
			c.SetStopped(platformConstructExitStatus(e))
			defer autoRemove()
		}

		daemon.updateHealthMonitor(c)
		attributes := map[string]string{
			"exitCode": strconv.Itoa(int(e.ExitCode)),
		}
		daemon.LogContainerEventWithAttributes(c, "die", attributes)
		daemon.Cleanup(c)

		if err == nil && restart {
			go func() {
				err := <-wait
				if err == nil {
					if err = daemon.containerStart(c, "", false); err != nil {
						logrus.Debugf("failed to restart contianer: %+v", err)
					}
				}
				if err != nil {
					c.SetStopped(platformConstructExitStatus(e))
					defer autoRemove()
					if err != restartmanager.ErrRestartCanceled {
						logrus.Errorf("restartmanger wait error: %+v", err)
					}
				}
			}()
		}

		defer c.Unlock()
		if err := c.ToDisk(); err != nil {
			return err
		}
		return daemon.postRunProcessing(c, e)
	case libcontainerd.StateExitProcess:
		c.Lock()
		defer c.Unlock()
		if execConfig := c.ExecCommands.Get(e.ProcessID); execConfig != nil {
			ec := int(e.ExitCode)
			execConfig.ExitCode = &ec
			execConfig.Running = false
			execConfig.Wait()
			if err := execConfig.CloseStreams(); err != nil {
				logrus.Errorf("%s: %s", c.ID, err)
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
		if err := c.ToDisk(); err != nil {
			c.Reset(false)
			return err
		}
		daemon.initHealthMonitor(c)
		daemon.LogContainerEvent(c, "start")
	case libcontainerd.StatePause:
		// Container is already locked in this case
		c.Paused = true
		if err := c.ToDisk(); err != nil {
			return err
		}
		daemon.updateHealthMonitor(c)
		daemon.LogContainerEvent(c, "pause")
	case libcontainerd.StateResume:
		// Container is already locked in this case
		c.Paused = false
		if err := c.ToDisk(); err != nil {
			return err
		}
		daemon.updateHealthMonitor(c)
		daemon.LogContainerEvent(c, "unpause")
	}

	return nil
}

// AttachStreams is called by libcontainerd to connect the stdio.
func (daemon *Daemon) AttachStreams(id string, iop libcontainerd.IOPipe) error {
	var (
		s  *runconfig.StreamConfig
		ec *exec.Config
	)

	c := daemon.containers.Get(id)
	if c == nil {
		var err error
		ec, err = daemon.getExecConfig(id)
		if err != nil {
			return fmt.Errorf("no such exec/container: %s", id)
		}
		s = ec.StreamConfig
	} else {
		s = c.StreamConfig
		if err := daemon.StartLogging(c); err != nil {
			c.Reset(false)
			return err
		}
	}

	copyFunc := func(w io.Writer, r io.Reader) {
		s.Add(1)
		go func() {
			if _, err := io.Copy(w, r); err != nil {
				logrus.Errorf("%v stream copy error: %v", id, err)
			}
			s.Done()
		}()
	}

	if iop.Stdout != nil {
		copyFunc(s.Stdout(), iop.Stdout)
	}
	if iop.Stderr != nil {
		copyFunc(s.Stderr(), iop.Stderr)
	}

	if stdin := s.Stdin(); stdin != nil {
		if iop.Stdin != nil {
			go func() {
				io.Copy(iop.Stdin, stdin)
				if err := iop.Stdin.Close(); err != nil {
					logrus.Error(err)
				}
			}()
		}
	} else {
		//TODO(swernli): On Windows, not closing stdin when no tty is requested by the exec Config
		// results in a hang. We should re-evaluate generalizing this fix for all OSes if
		// we can determine that is the right thing to do more generally.
		if (c != nil && !c.Config.Tty) || (ec != nil && !ec.Tty && runtime.GOOS == "windows") {
			// tty is enabled, so dont close containerd's iopipe stdin.
			if iop.Stdin != nil {
				if err := iop.Stdin.Close(); err != nil {
					logrus.Error(err)
				}
			}
		}
	}

	return nil
}
