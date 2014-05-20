// +build !linux

package nodes

import "github.com/dotcloud/docker/pkg/libcontainer"

var DefaultNodes = []string{}

func GetHostDeviceNodes() ([]string, error) {
	return nil, libcontainer.ErrUnsupported
}
