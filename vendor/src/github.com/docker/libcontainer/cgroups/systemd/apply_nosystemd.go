// +build !linux

package systemd

import (
	"fmt"

	"github.com/docker/libcontainer/cgroups"
)

func UseSystemd() bool {
	return false
}

func Apply(c *cgroups.Cgroup, pid int) (cgroups.ActiveCgroup, error) {
	return nil, fmt.Errorf("Systemd not supported")
}

func GetPids(c *cgroups.Cgroup) ([]int, error) {
	return nil, fmt.Errorf("Systemd not supported")
}

func Freeze(c *cgroups.Cgroup, state cgroups.FreezerState) error {
	return fmt.Errorf("Systemd not supported")
}

func GetStats(c *cgroups.Cgroup) (*cgroups.Stats, error) {
	return nil, fmt.Errorf("Systemd not supported")
}
