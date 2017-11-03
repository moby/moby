package daemon

import (
	"github.com/moby/moby/container"
	"github.com/moby/moby/libcontainerd"
)

// postRunProcessing perfoms any processing needed on the container after it has stopped.
func (daemon *Daemon) postRunProcessing(_ *container.Container, _ libcontainerd.EventInfo) error {
	return nil
}
