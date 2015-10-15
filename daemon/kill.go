package daemon

import (
	"fmt"
	"runtime"
	"syscall"

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
		if err := container.Kill(); err != nil {
			return err
		}
	} else {
		// Otherwise, just send the requested signal
		if err := container.killSig(int(sig)); err != nil {
			return err
		}
	}
	return nil
}
