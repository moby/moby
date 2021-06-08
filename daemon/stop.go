package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"time"

	containerpkg "github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ContainerStop looks for the given container and stops it.
// In case the container fails to stop gracefully within a time duration
// specified by the timeout argument, in seconds, it is forcefully
// terminated (killed).
//
// If the timeout is nil, the container's StopTimeout value is used, if set,
// otherwise the engine default. A negative timeout value can be specified,
// meaning no timeout, i.e. no forceful termination is performed.
func (daemon *Daemon) ContainerStop(name string, timeout *int) error {
	container, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}
	if !container.IsRunning() {
		return containerNotModifiedError{running: false}
	}
	if timeout == nil {
		stopTimeout := container.StopTimeout()
		timeout = &stopTimeout
	}
	if err := daemon.containerStop(container, *timeout); err != nil {
		return errdefs.System(errors.Wrapf(err, "cannot stop container: %s", name))
	}
	return nil
}

// containerStop sends a stop signal, waits, sends a kill signal.
func (daemon *Daemon) containerStop(container *containerpkg.Container, seconds int) error {
	// TODO propagate a context down to this function
	ctx := context.TODO()
	if !container.IsRunning() {
		return nil
	}
	var wait time.Duration
	if seconds >= 0 {
		wait = time.Duration(seconds) * time.Second
	}
	success := func() error {
		daemon.LogContainerEvent(container, "stop")
		return nil
	}
	stopSignal := container.StopSignal()

	// 1. Send a stop signal
	err := daemon.killPossiblyDeadProcess(container, stopSignal)
	if err != nil {
		wait = 2 * time.Second
	}

	var subCtx context.Context
	var cancel context.CancelFunc
	if seconds >= 0 {
		subCtx, cancel = context.WithTimeout(ctx, wait)
	} else {
		subCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	if status := <-container.Wait(subCtx, containerpkg.WaitConditionNotRunning); status.Err() == nil {
		// container did exit, so ignore any previous errors and return
		return success()
	}

	if err != nil {
		// the container has still not exited, and the kill function errored, so log the error here:
		logrus.WithError(err).WithField("container", container.ID).Errorf("Error sending stop (signal %d) to container", stopSignal)
	}
	if seconds < 0 {
		// if the client requested that we never kill / wait forever, but container.Wait was still
		// interrupted (parent context cancelled, for example), we should propagate the signal failure
		return err
	}

	logrus.WithField("container", container.ID).Infof("Container failed to exit within %s of signal %d - using the force", wait, stopSignal)
	// Stop either failed or container didnt exit, so fallback to kill.
	if err := daemon.Kill(container); err != nil {
		// got a kill error, but give container 2 more seconds to exit just in case
		subCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if status := <-container.Wait(subCtx, containerpkg.WaitConditionNotRunning); status.Err() == nil {
			// container did exit, so ignore error and return
			return success()
		}
		logrus.WithError(err).WithField("container", container.ID).Error("Error killing the container")
		return err
	}

	return success()
}
