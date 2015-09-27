// +build windows

package windows

import (
	"github.com/docker/docker/daemon/execdriver"
	derr "github.com/docker/docker/errors"
)

// Stats implements the exec driver Driver interface.
func (d *Driver) Stats(id string) (*execdriver.ResourceStats, error) {
	return nil, derr.ErrorCodeWinStatsNotImpl
}
