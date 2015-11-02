package daemon

// logContainerEvent generates an event related to a container.
func (daemon *Daemon) logContainerEvent(container *Container, action string) {
	daemon.EventsService.Log(
		action,
		container.ID,
		container.Config.Image,
	)
}
