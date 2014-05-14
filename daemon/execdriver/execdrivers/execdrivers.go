package execdrivers

import (
	"fmt"
	"github.com/dotcloud/docker/daemon/execdriver"
	"github.com/dotcloud/docker/daemon/execdriver/native"
	"github.com/dotcloud/docker/pkg/sysinfo"
	"path"
)

func NewDriver(name, root, initPath string, sysInfo *sysinfo.SysInfo) (execdriver.Driver, error) {
	switch name {
	case "native":
		return native.NewDriver(path.Join(root, "execdriver", "native"), initPath)
	}
	return nil, fmt.Errorf("unknown exec driver %s", name)
}
