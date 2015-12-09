package daemon

import (
	"github.com/docker/docker/container"
)

// LogContainerEvent generates an event related to a container.
func (daemon *Daemon) LogContainerEvent(container *container.Container, action string) {
	daemon.EventsService.Log(
		action,
		container.ID,
		container.Config.Image,
	)
}
