// +build !linux

package nsinit

import (
	"github.com/dotcloud/docker/pkg/libcontainer"
)

func GetNamespaceFlags(namespaces libcontainer.Namespaces) (flag int) {
	return 0
}
