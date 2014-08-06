package daemon

import (
	"strconv"
	"strings"
	"syscall"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/signal"
)

// ContainerKill send signal to the container
// If no signal is given (sig 0), then Kill with SIGKILL and wait
// for the container to exit.
// If a signal is given, then just send it to the container and return.
func (daemon *Daemon) ContainerKill(job *engine.Job) engine.Status {
	if n := len(job.Args); n < 1 || n > 2 {
		return job.Errorf("Usage: %s CONTAINER [SIGNAL]", job.Name)
	}
	var (
		name = job.Args[0]
		sig  uint64
		err  error
	)

	// If we have a signal, look at it. Otherwise, do nothing
	if len(job.Args) == 2 && job.Args[1] != "" {
		// Check if we passed the signal as a number:
		// The largest legal signal is 31, so let's parse on 5 bits
		sig, err = strconv.ParseUint(job.Args[1], 10, 5)
		if err != nil {
			// The signal is not a number, treat it as a string (either like "KILL" or like "SIGKILL")
			sig = uint64(signal.SignalMap[strings.TrimPrefix(job.Args[1], "SIG")])
		}

		if sig == 0 {
			return job.Errorf("Invalid signal: %s", job.Args[1])
		}
	}

	if container := daemon.Get(name); container != nil {
		// If no signal is passed, or SIGKILL, perform regular Kill (SIGKILL + wait())
		if sig == 0 || syscall.Signal(sig) == syscall.SIGKILL {
			if err := container.Kill(); err != nil {
				return job.Errorf("Cannot kill container %s: %s", name, err)
			}
			container.LogEvent("kill")
		} else {
			// Otherwise, just send the requested signal
			if err := container.KillSig(int(sig)); err != nil {
				return job.Errorf("Cannot kill container %s: %s", name, err)
			}
			// FIXME: Add event for signals
		}
	} else {
		return job.Errorf("No such container: %s", name)
	}
	return engine.StatusOK
}
