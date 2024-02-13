package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"time"

	"github.com/containerd/log"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/internal/compatcontext"
	"github.com/moby/sys/signal"
	"github.com/pkg/errors"
)

// ContainerStop looks for the given container and stops it.
// In case the container fails to stop gracefully within a time duration
// specified by the timeout argument, in seconds, it is forcefully
// terminated (killed).
//
// If the timeout is nil, the container's StopTimeout value is used, if set,
// otherwise the engine default. A negative timeout value can be specified,
// meaning no timeout, i.e. no forceful termination is performed.
func (daemon *Daemon) ContainerStop(ctx context.Context, name string, options containertypes.StopOptions) error {
	ctr, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}
	if !ctr.IsRunning() {
		// This is not an actual error, but produces a 304 "not modified"
		// when returned through the API to indicates the container is
		// already in the desired state. It's implemented as an error
		// to make the code calling this function terminate early (as
		// no further processing is needed).
		return errdefs.NotModified(errors.New("container is already stopped"))
	}
	err = daemon.containerStop(ctx, ctr, options)
	if err != nil {
		return errdefs.System(errors.Wrapf(err, "cannot stop container: %s", name))
	}
	return nil
}

// containerStop sends a stop signal, waits, sends a kill signal. It uses
// a [context.WithoutCancel], so cancelling the context does not cancel
// the request to stop the container.
func (daemon *Daemon) containerStop(ctx context.Context, ctr *container.Container, options containertypes.StopOptions) (retErr error) {
	// Cancelling the request should not cancel the stop.
	ctx = compatcontext.WithoutCancel(ctx)

	if !ctr.IsRunning() {
		return nil
	}

	var (
		stopSignal  = ctr.StopSignal()
		stopTimeout = ctr.StopTimeout()
	)
	if options.Signal != "" {
		sig, err := signal.ParseSignal(options.Signal)
		if err != nil {
			return errdefs.InvalidParameter(err)
		}
		stopSignal = sig
	}
	if options.Timeout != nil {
		stopTimeout = *options.Timeout
	}

	var wait time.Duration
	if stopTimeout >= 0 {
		wait = time.Duration(stopTimeout) * time.Second
	}
	defer func() {
		if retErr == nil {
			daemon.LogContainerEvent(ctr, events.ActionStop)
		}
	}()

	// 1. Send a stop signal
	err := daemon.killPossiblyDeadProcess(ctr, stopSignal)
	if err != nil {
		wait = 2 * time.Second
	}

	var subCtx context.Context
	var cancel context.CancelFunc
	if stopTimeout >= 0 {
		subCtx, cancel = context.WithTimeout(ctx, wait)
	} else {
		subCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	if status := <-ctr.Wait(subCtx, container.WaitConditionNotRunning); status.Err() == nil {
		// container did exit, so ignore any previous errors and return
		return nil
	}

	if err != nil {
		// the container has still not exited, and the kill function errored, so log the error here:
		log.G(ctx).WithError(err).WithField("container", ctr.ID).Errorf("Error sending stop (signal %d) to container", stopSignal)
	}
	if stopTimeout < 0 {
		// if the client requested that we never kill / wait forever, but container.Wait was still
		// interrupted (parent context cancelled, for example), we should propagate the signal failure
		return err
	}

	log.G(ctx).WithField("container", ctr.ID).Infof("Container failed to exit within %s of signal %d - using the force", wait, stopSignal)

	// Stop either failed or container didn't exit, so fallback to kill.
	if err := daemon.Kill(ctr); err != nil {
		// got a kill error, but give container 2 more seconds to exit just in case
		subCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		status := <-ctr.Wait(subCtx, container.WaitConditionNotRunning)
		if status.Err() != nil {
			log.G(ctx).WithError(err).WithField("container", ctr.ID).Errorf("error killing container: %v", status.Err())
			return err
		}
		// container did exit, so ignore previous errors and continue
	}

	return nil
}
