package daemon

import "time"

// ContainerWait stops processing until the given container is
// stopped. If the container is not found, an error is returned. On a
// successful stop, the exit code of the container is returned. On a
// timeout, an error is returned. If you want to wait forever, supply
// a negative duration for the timeout.
func (daemon *Daemon) ContainerWait(name string, timeout time.Duration) (int, error) {
	container, err := daemon.Get(name)
	if err != nil {
		return -1, err
	}

	return container.WaitStop(timeout)
}
