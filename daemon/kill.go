package daemon

import (
	"syscall"

	"github.com/docker/docker/context"
)

// ContainerKill send signal to the container
// If no signal is given (sig 0), then Kill with SIGKILL and wait
// for the container to exit.
// If a signal is given, then just send it to the container and return.
func (daemon *Daemon) ContainerKill(ctx context.Context, name string, sig uint64) error {
	container, err := daemon.Get(ctx, name)
	if err != nil {
		return err
	}

	// If no signal is passed, or SIGKILL, perform regular Kill (SIGKILL + wait())
	if sig == 0 || syscall.Signal(sig) == syscall.SIGKILL {
		if err := container.Kill(ctx); err != nil {
			return err
		}
	} else {
		// Otherwise, just send the requested signal
		if err := container.killSig(ctx, int(sig)); err != nil {
			return err
		}
	}
	return nil
}
