package daemon

import "fmt"

// ContainerStop looks for the given container and terminates it,
// waiting the given number of seconds before forcefully killing the
// container. If a negative number of seconds is given, ContainerStop
// will wait for a graceful termination. An error is returned if the
// container is not found, is already stopped, or if there is a
// problem stopping the container.
func (daemon *Daemon) ContainerStop(container *Container, seconds int) error {
	if !container.IsRunning() {
		return fmt.Errorf("Container already stopped")
	}
	if err := container.Stop(seconds); err != nil {
		return fmt.Errorf("Cannot stop container %s: %s\n", container.ID, err)
	}
	return nil
}
