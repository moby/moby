//+build windows

package daemon

import (
	"github.com/moby/moby/container"
)

func (daemon *Daemon) saveApparmorConfig(container *container.Container) error {
	return nil
}
