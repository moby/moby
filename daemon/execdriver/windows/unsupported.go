// +build !windows

package windows

import (
	"fmt"

	"github.com/docker/docker/daemon/execdriver"
	derr "github.com/docker/docker/errors"
)

// NewDriver returns a new execdriver.Driver
func NewDriver(root, initPath string) (execdriver.Driver, error) {
	return nil, derr.ErrorCodeWinDriverNotSupported
}
