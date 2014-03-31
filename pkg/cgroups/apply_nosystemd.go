// +build !linux

package cgroups

import (
	"fmt"
)

func useSystemd() bool {
	return false
}

func systemdApply(c *Cgroup, pid int) (ActiveCgroup, error) {
	return nil, fmt.Errorf("Systemd not supported")
}
