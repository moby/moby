// +build !linux

package nsinit

import (
	"github.com/dotcloud/docker/pkg/libcontainer"
)

func (ns *linuxNs) Exec(container *libcontainer.Container, term Terminal, pidRoot string, args []string, startCallback func()) (int, error) {
	return -1, libcontainer.ErrUnsupported
}

func (ns *linuxNs) ExecIn(container *libcontainer.Container, nspid int, args []string) (int, error) {
	return -1, libcontainer.ErrUnsupported
}

func (ns *linuxNs) Init(container *libcontainer.Container, uncleanRootfs, console string, syncPipe *SyncPipe, args []string) error {
	return libcontainer.ErrUnsupported
}
