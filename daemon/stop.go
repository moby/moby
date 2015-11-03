package daemon

import (
	"time"

	"github.com/Sirupsen/logrus"
	derr "github.com/docker/docker/errors"
)

// ContainerStop looks for the given container and terminates it,
// waiting the given number of seconds before forcefully killing the
// container. If a negative number of seconds is given, ContainerStop
// will wait for a graceful termination. An error is returned if the
// container is not found, is already stopped, or if there is a
// problem stopping the container.
func (daemon *Daemon) ContainerStop(name string, seconds int) error {
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}
	if !container.IsRunning() {
		return derr.ErrorCodeStopped
	}
	if err := daemon.containerStop(container, seconds); err != nil {
		return derr.ErrorCodeCantStop.WithArgs(name, err)
	}
	return nil
}

// containerStop halts a container by sending a stop signal, waiting for the given
// duration in seconds, and then calling SIGKILL and waiting for the
// process to exit. If a negative duration is given, Stop will wait
// for the initial signal forever. If the container is not running Stop returns
// immediately.
func (daemon *Daemon) containerStop(container *Container, seconds int) error {
	if !container.IsRunning() {
		return nil
	}

	// 1. Send a SIGTERM
	if err := daemon.killPossiblyDeadProcess(container, container.stopSignal()); err != nil {
		logrus.Infof("Failed to send SIGTERM to the process, force killing")
		if err := daemon.killPossiblyDeadProcess(container, 9); err != nil {
			return err
		}
	}

	// 2. Wait for the process to exit on its own
	if _, err := container.WaitStop(time.Duration(seconds) * time.Second); err != nil {
		logrus.Infof("Container %v failed to exit within %d seconds of SIGTERM - using the force", container.ID, seconds)
		// 3. If it doesn't, then send SIGKILL
		if err := daemon.Kill(container); err != nil {
			container.WaitStop(-1 * time.Second)
			logrus.Warn(err) // Don't return error because we only care that container is stopped, not what function stopped it
		}
	}

	daemon.LogContainerEvent(container, "stop")
	return nil
}
