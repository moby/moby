// +build windows

package windows

import (
	"github.com/docker/docker/daemon/execdriver"
	derr "github.com/docker/docker/errors"
)

// Pause implements the exec driver Driver interface.
func (d *Driver) Pause(c *execdriver.Command) error {
	return derr.ErrorCodeWinErrPauseUnpause
}

// Unpause implements the exec driver Driver interface.
func (d *Driver) Unpause(c *execdriver.Command) error {
	return derr.ErrorCodeWinErrPauseUnpause
}
