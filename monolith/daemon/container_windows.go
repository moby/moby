//+build windows

package daemon

import (
	"github.com/docker/docker/monolith/container"
)

func (daemon *Daemon) saveApparmorConfig(container *container.Container) error {
	return nil
}
