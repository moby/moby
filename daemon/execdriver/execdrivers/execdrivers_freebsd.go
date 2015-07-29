// +build freebsd

package execdrivers

import (
	"fmt"

	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/pkg/sysinfo"
)

// NewDriver returns a new execdriver.Driver from the given name configured with the provided options.
func NewDriver(name string, options []string, root, libPath, initPath string, sysInfo *sysinfo.SysInfo) (execdriver.Driver, error) {
	switch name {
	case "jail":
		return nil, fmt.Errorf("jail driver not yet supported on FreeBSD")
	}
	return nil, fmt.Errorf("unknown exec driver %s", name)
}
