//go:build !linux

package daemon

import (
	"github.com/moby/moby/v2/daemon/container"
)

// saveAppArmorConfig is a no-op on platforms without AppArmor support.
func (daemon *Daemon) saveAppArmorConfig(*container.Container) error {
	return nil
}
