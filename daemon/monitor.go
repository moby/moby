package daemon

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/libcontainerd"
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
		c.Lock()
		defer c.Unlock()
		c.Wait()
		c.Reset(false)
		c.SetStopped(platformConstructExitStatus(e))
		attributes := map[string]string{
			"exitCode": strconv.Itoa(int(e.ExitCode)),
		}
		daemon.updateHealthMonitor(c)
		daemon.LogContainerEventWithAttributes(c, "die", attributes)
		daemon.Cleanup(c)
		// FIXME: here is race condition between two RUN instructions in Dockerfile
		// because they share same runconfig and change image. Must be fixed
		// in builder/builder.go
		if err := c.ToDisk(); err != nil {
			return err
		}
		return daemon.postRunProcessing(c, e)
	case libcontainerd.StateRestart:
		c.Lock()
		defer c.Unlock()
		c.Reset(false)
		c.RestartCount++
		c.SetRestarting(platformConstructExitStatus(e))
		attributes := map[string]string{
			"exitCode": strconv.Itoa(int(e.ExitCode)),
		}
		daemon.LogContainerEventWithAttributes(c, "die", attributes)
		daemon.updateHealthMonitor(c)
		return c.ToDisk()
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
		if err := c.ToDisk(); err != nil {
			c.Reset(false)
			return err
		}
		daemon.initHealthMonitor(c)
		daemon.LogContainerEvent(c, "start")
	case libcontainerd.StatePause:
		// Container is already locked in this case
		c.Paused = true
		daemon.updateHealthMonitor(c)
		daemon.LogContainerEvent(c, "pause")
	case libcontainerd.StateResume:
		// Container is already locked in this case
		c.Paused = false
		daemon.updateHealthMonitor(c)
		daemon.LogContainerEvent(c, "unpause")
	}

	return nil
}

// AttachStreams is called by libcontainerd to connect the stdio.
func (daemon *Daemon) AttachStreams(id string, iop libcontainerd.IOPipe) error {
	var s *runconfig.StreamConfig
	c := daemon.containers.Get(id)
	if c == nil {
		ec, err := daemon.getExecConfig(id)
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

	if stdin := s.Stdin(); stdin != nil {
		if iop.Stdin != nil {
			go func() {
				io.Copy(iop.Stdin, stdin)
				iop.Stdin.Close()
			}()
		}
	} else {
		if c != nil && !c.Config.Tty {
			// tty is enabled, so dont close containerd's iopipe stdin.
			if iop.Stdin != nil {
				iop.Stdin.Close()
			}
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

	return nil
}
