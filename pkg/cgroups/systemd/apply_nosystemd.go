// +build !linux

package systemd

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/cgroups"
)

func UseSystemd() bool {
	return false
}

func systemdApply(c *Cgroup, pid int) (cgroups.ActiveCgroup, error) {
	return nil, fmt.Errorf("Systemd not supported")
}
