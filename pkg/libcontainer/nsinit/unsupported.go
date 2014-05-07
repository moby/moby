// +build !linux

package nsinit

import (
	"github.com/dotcloud/docker/pkg/cgroups"
	"github.com/dotcloud/docker/pkg/libcontainer"
)

func Exec(container *libcontainer.Container, term Terminal, rootfs, dataPath string, args []string, createCommand CreateCommand, startCallback func()) (int, error) {
	return -1, libcontainer.ErrUnsupported
}

func Init(container *libcontainer.Container, uncleanRootfs, consolePath string, syncPipe *SyncPipe, args []string) error {
	return libcontainer.ErrUnsupported
}

func InitializeNetworking(container *libcontainer.Container, nspid int, pipe *SyncPipe) error {
	return libcontainer.ErrUnsupported
}

func SetupCgroups(container *libcontainer.Container, nspid int) (cgroups.ActiveCgroup, error) {
	return nil, libcontainer.ErrUnsupported
}

func GetNamespaceFlags(namespaces map[string]bool) (flag int) {
	return 0
}
