package daemon

// LogContainerEvent generates an event related to a container.
func (daemon *Daemon) LogContainerEvent(container *Container, action string) {
	daemon.EventsService.Log(
		action,
		container.ID,
		container.Config.Image,
	)
}
