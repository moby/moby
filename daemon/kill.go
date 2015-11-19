package daemon

import (
	"fmt"
	"runtime"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/pkg/signal"
)

// ContainerKill send signal to the container
// If no signal is given (sig 0), then Kill with SIGKILL and wait
// for the container to exit.
// If a signal is given, then just send it to the container and return.
func (daemon *Daemon) ContainerKill(name string, sig uint64) error {
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	if sig != 0 && !signal.ValidSignalForPlatform(syscall.Signal(sig)) {
		return fmt.Errorf("The %s daemon does not support signal %d", runtime.GOOS, sig)
	}

	// If no signal is passed, or SIGKILL, perform regular Kill (SIGKILL + wait())
	if sig == 0 || syscall.Signal(sig) == syscall.SIGKILL {
		return daemon.Kill(container)
	}
	return daemon.killWithSignal(container, int(sig))
}

// killWithSignal sends the container the given signal. This wrapper for the
// host specific kill command prepares the container before attempting
// to send the signal. An error is returned if the container is paused
// or not running, or if there is a problem returned from the
// underlying kill command.
func (daemon *Daemon) killWithSignal(container *Container, sig int) error {
	logrus.Debugf("Sending %d to %s", sig, container.ID)
	container.Lock()
	defer container.Unlock()

	// We could unpause the container for them rather than returning this error
	if container.Paused {
		return derr.ErrorCodeUnpauseContainer.WithArgs(container.ID)
	}

	if !container.Running {
		return derr.ErrorCodeNotRunning.WithArgs(container.ID)
	}

	container.ExitOnNext()

	// if the container is currently restarting we do not need to send the signal
	// to the process.  Telling the monitor that it should exit on it's next event
	// loop is enough
	if container.Restarting {
		return nil
	}

	if err := daemon.kill(container, sig); err != nil {
		return err
	}

	daemon.LogContainerEvent(container, "kill")
	return nil
}

// Kill forcefully terminates a container.
func (daemon *Daemon) Kill(container *Container) error {
	if !container.IsRunning() {
		return derr.ErrorCodeNotRunning.WithArgs(container.ID)
	}

	// 1. Send SIGKILL
	if err := daemon.killPossiblyDeadProcess(container, int(syscall.SIGKILL)); err != nil {
		// While normally we might "return err" here we're not going to
		// because if we can't stop the container by this point then
		// its probably because its already stopped. Meaning, between
		// the time of the IsRunning() call above and now it stopped.
		// Also, since the err return will be exec driver specific we can't
		// look for any particular (common) error that would indicate
		// that the process is already dead vs something else going wrong.
		// So, instead we'll give it up to 2 more seconds to complete and if
		// by that time the container is still running, then the error
		// we got is probably valid and so we return it to the caller.

		if container.IsRunning() {
			container.WaitStop(2 * time.Second)
			if container.IsRunning() {
				return err
			}
		}
	}

	// 2. Wait for the process to die, in last resort, try to kill the process directly
	if err := killProcessDirectly(container); err != nil {
		return err
	}

	container.WaitStop(-1 * time.Second)
	return nil
}

// killPossibleDeadProcess is a wrapper aroung killSig() suppressing "no such process" error.
func (daemon *Daemon) killPossiblyDeadProcess(container *Container, sig int) error {
	err := daemon.killWithSignal(container, sig)
	if err == syscall.ESRCH {
		logrus.Debugf("Cannot kill process (pid=%d) with signal %d: no such process.", container.GetPID(), sig)
		return nil
	}
	return err
}
