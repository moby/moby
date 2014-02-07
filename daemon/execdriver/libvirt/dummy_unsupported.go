// Dummy file to include if not otherwise building libvirt driver
// Include on non-Linux, or if static binary (libvirt doesn't like static linking)
// +build !linux

package libvirt

import (
	"fmt"
	"github.com/docker/docker/daemon/execdriver"
)

func NewDriver(root string) (execdriver.Driver, error) {
	return nil, fmt.Errorf("libvirt backend not supported")
}
