package execdrivers

import (
	"fmt"
	"github.com/dotcloud/docker/execdriver"
	"github.com/dotcloud/docker/execdriver/lxc"
	"github.com/dotcloud/docker/execdriver/native"
	"github.com/dotcloud/docker/pkg/sysinfo"
	"path"
)

func NewDriver(name, root string, sysInfo *sysinfo.SysInfo) (execdriver.Driver, error) {
	switch name {
	case "lxc":
		// we want to five the lxc driver the full docker root because it needs
		// to access and write config and template files in /var/lib/docker/containers/*
		// to be backwards compatible
		return lxc.NewDriver(root, sysInfo.AppArmor)
	case "native":
		return native.NewDriver(path.Join(root, "execdriver", "native"))
	}
	return nil, fmt.Errorf("unknown exec driver %s", name)
}
