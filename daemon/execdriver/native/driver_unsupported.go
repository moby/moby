// +build !linux

package native

import (
	"fmt"

	"github.com/docker/docker/daemon/execdriver"
	derr "github.com/docker/docker/errors"
)

// NewDriver returns a new native driver, called from NewDriver of execdriver.
func NewDriver(root, initPath string) (execdriver.Driver, error) {
	return nil, derr.ErrorCodeNativeDriverNotSupported
}
