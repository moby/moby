// +build windows

package execdrivers

import (
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/daemon/execdriver/windows"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/pkg/sysinfo"
)

// NewDriver returns a new execdriver.Driver from the given name configured with the provided options.
func NewDriver(name string, options []string, root, libPath, initPath string, sysInfo *sysinfo.SysInfo) (execdriver.Driver, error) {
	switch name {
	case "windows":
		return windows.NewDriver(root, initPath, options)
	}
	return nil, derr.ErrorCodeExecDriverUnknown.WithArgs(name)
}
