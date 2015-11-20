// +build linux

package execdrivers

import (
	"fmt"
	"path"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/daemon/execdriver/clr"
	"github.com/docker/docker/daemon/execdriver/lxc"
	"github.com/docker/docker/daemon/execdriver/native"
	"github.com/docker/docker/pkg/sysinfo"
)

// NewDriver returns a new execdriver.Driver from the given name configured with the provided options.
func NewDriver(name string, options []string, root, libPath, initPath string, sysInfo *sysinfo.SysInfo) (execdriver.Driver, error) {
	rootPath := path.Join(root, "execdriver", name)
	switch name {
	case "clr":
		return clr.NewDriver(rootPath, libPath, initPath, sysInfo.AppArmor)
	case "lxc":
		// we want to give the lxc driver the full docker root because it needs
		// to access and write config and template files in /var/lib/docker/containers/*
		// to be backwards compatible
		logrus.Warn("LXC built-in support is deprecated.")
		return lxc.NewDriver(root, libPath, initPath, sysInfo.AppArmor)
	case "native":
		return native.NewDriver(rootPath, initPath, options)
	}
	return nil, fmt.Errorf("unknown exec driver %s", name)
}
