// +build !linux

package native

import (
	"fmt"

	"github.com/docker/docker/daemon/execdriver"
)

// NewDriver returns a new native driver, called from NewDriver of execdriver.
func NewDriver(root, initPath string) (execdriver.Driver, error) {
	return nil, fmt.Errorf("native driver not supported on non-linux")
}
