// +build linux

package execdrivers

import (
	"fmt"
	"path"

	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/daemon/execdriver/native"
	"github.com/docker/docker/pkg/sysinfo"
)

// NewDriver returns a new execdriver.Driver from the given name configured with the provided options.
func NewDriver(name string, options []string, root, libPath, initPath string, sysInfo *sysinfo.SysInfo) (execdriver.Driver, error) {
	if name != "native" {
		return nil, fmt.Errorf("unknown exec driver %s", name)
	}
	return native.NewDriver(path.Join(root, "execdriver", "native"), initPath, options)
}
