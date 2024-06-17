package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/containerd/log"
	"github.com/docker/docker/api/types/events"
	containerpkg "github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/moby/sys/signal"
	"github.com/pkg/errors"
)

type errNoSuchProcess struct {
	pid    int
	signal syscall.Signal
}

func (e errNoSuchProcess) Error() string {
	return fmt.Sprintf("cannot kill process (pid=%d) with signal %d: no such process", e.pid, e.signal)
}

func (errNoSuchProcess) NotFound() {}

// ContainerKill sends signal to the container
// If no signal is given, then Kill with SIGKILL and wait
// for the container to exit.
// If a signal is given, then just send it to the container and return.
func (daemon *Daemon) ContainerKill(name, stopSignal string) error {
	var (
		err error
		sig = syscall.SIGKILL
	)
	if stopSignal != "" {
		sig, err = signal.ParseSignal(stopSignal)
		if err != nil {
			return errdefs.InvalidParameter(err)
		}
		if !signal.ValidSignalForPlatform(sig) {
			return errdefs.InvalidParameter(errors.Errorf("the %s daemon does not support signal %d", runtime.GOOS, sig))
		}
	}
	container, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}
	if sig == syscall.SIGKILL {
		// perform regular Kill (SIGKILL + wait())
		return daemon.Kill(container)
	}
	return daemon.killWithSignal(container, sig)
}

// killWithSignal sends the container the given signal. This wrapper for the
// host specific kill command prepares the container before attempting
// to send the signal. An error is returned if the container is paused
// or not running, or if there is a problem returned from the
// underlying kill command.
func (daemon *Daemon) killWithSignal(container *containerpkg.Container, stopSignal syscall.Signal) error {
	log.G(context.TODO()).Debugf("Sending kill signal %d to container %s", stopSignal, container.ID)
	container.Lock()
	defer container.Unlock()

	task, err := container.GetRunningTask()
	if err != nil {
		return err
	}

	var unpause bool
	if container.Config.StopSignal != "" && stopSignal != syscall.SIGKILL {
		containerStopSignal, err := signal.ParseSignal(container.Config.StopSignal)
		if err != nil {
			return err
		}
		if containerStopSignal == stopSignal {
			container.ExitOnNext()
			unpause = container.Paused
		}
	} else {
		container.ExitOnNext()
		unpause = container.Paused
	}

	if !daemon.IsShuttingDown() {
		container.HasBeenManuallyStopped = true
		if err := container.CheckpointTo(context.WithoutCancel(context.TODO()), daemon.containersReplica); err != nil {
			log.G(context.TODO()).WithFields(log.Fields{
				"error":     err,
				"container": container.ID,
			}).Warn("error checkpointing container state")
		}
	}

	// if the container is currently restarting we do not need to send the signal
	// to the process. Telling the monitor that it should exit on its next event
	// loop is enough
	if container.Restarting {
		return nil
	}

	if err := task.Kill(context.Background(), stopSignal); err != nil {
		if errdefs.IsNotFound(err) {
			unpause = false
			log.G(context.TODO()).WithError(err).WithField("container", container.ID).WithField("action", "kill").Debug("container kill failed because of 'container not found' or 'no such process'")
			go func() {
				// We need to clean up this container but it is possible there is a case where we hit here before the exit event is processed
				// but after it was fired off.
				// So let's wait the container's stop timeout amount of time to see if the event is eventually processed.
				// Doing this has the side effect that if no event was ever going to come we are waiting a longer period of time unnecessarily.
				// But this prevents race conditions in processing the container.
				ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(container.StopTimeout())*time.Second)
				defer cancel()
				s := <-container.Wait(ctx, containerpkg.WaitConditionNotRunning)
				if s.Err() != nil {
					if err := daemon.handleContainerExit(container, nil); err != nil {
						log.G(context.TODO()).WithFields(log.Fields{
							"error":     err,
							"container": container.ID,
							"action":    "kill",
						}).Warn("error while handling container exit")
					}
				}
			}()
		} else {
			return errors.Wrapf(err, "Cannot kill container %s", container.ID)
		}
	}

	if unpause {
		// above kill signal will be sent once resume is finished
		if err := task.Resume(context.Background()); err != nil {
			log.G(context.TODO()).Warnf("Cannot unpause container %s: %s", container.ID, err)
		}
	}

	daemon.LogContainerEventWithAttributes(container, events.ActionKill, map[string]string{
		"signal": strconv.Itoa(int(stopSignal)),
	})
	return nil
}

// Kill forcefully terminates a container.
func (daemon *Daemon) Kill(container *containerpkg.Container) error {
	if !container.IsRunning() {
		return errNotRunning(container.ID)
	}

	// 1. Send SIGKILL
	if err := daemon.killPossiblyDeadProcess(container, syscall.SIGKILL); err != nil {
		// kill failed, check if process is no longer running.
		if errors.As(err, &errNoSuchProcess{}) {
			return nil
		}
	}

	waitTimeout := 10 * time.Second
	if runtime.GOOS == "windows" {
		waitTimeout = 75 * time.Second // runhcs can be sloooooow.
	}

	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()

	status := <-container.Wait(ctx, containerpkg.WaitConditionNotRunning)
	if status.Err() == nil {
		return nil
	}

	log.G(ctx).WithFields(log.Fields{"error": status.Err(), "container": container.ID}).Warnf("Container failed to exit within %v of kill - trying direct SIGKILL", waitTimeout)

	if err := killProcessDirectly(container); err != nil {
		if errors.As(err, &errNoSuchProcess{}) {
			return nil
		}
		return err
	}

	// wait for container to exit one last time, if it doesn't then kill didnt work, so return error
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()

	if status := <-container.Wait(ctx2, containerpkg.WaitConditionNotRunning); status.Err() != nil {
		return errors.New("tried to kill container, but did not receive an exit event")
	}
	return nil
}

// killPossiblyDeadProcess is a wrapper around killSig() suppressing "no such process" error.
func (daemon *Daemon) killPossiblyDeadProcess(container *containerpkg.Container, sig syscall.Signal) error {
	err := daemon.killWithSignal(container, sig)
	if errdefs.IsNotFound(err) {
		err = errNoSuchProcess{container.GetPID(), sig}
		log.G(context.TODO()).Debug(err)
		return err
	}
	return err
}
