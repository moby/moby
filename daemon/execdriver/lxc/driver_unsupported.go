// +build !linux !amd64

package lxc

import (
	"fmt"

	"github.com/docker/docker/daemon/execdriver"
)

func NewDriver(root, initPath string, apparmor bool) (execdriver.Driver, error) {
	return nil, fmt.Errorf("lxc driver not supported on non-linux")
}

func KillLxc(id string, sig int) error {
	return fmt.Errorf("not implemented on non-linux")
}
