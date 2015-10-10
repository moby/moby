// +build windows

package windows

import (
	derr "github.com/docker/docker/errors"
)

// GetPidsForContainer implements the exec driver Driver interface.
func (d *Driver) GetPidsForContainer(id string) ([]int, error) {
	// TODO Windows: Implementation required.
	return nil, derr.ErrorCodeWinErrGetPid
}
