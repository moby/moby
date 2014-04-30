package nsinit

import "github.com/dotcloud/docker/pkg/libcontainer"

// NsInit is an interface with the public facing methods to provide high level
// exec operations on a container
type NsInit interface {
	Exec(container *libcontainer.Container, term Terminal, args []string) (int, error)
	ExecIn(container *libcontainer.Container, nspid int, args []string) (int, error)
	Init(container *libcontainer.Container, uncleanRootfs, console string, syncPipe *SyncPipe, args []string) error
}

type linuxNs struct {
	root           string
	commandFactory CommandFactory
	stateWriter    StateWriter
}

func NewNsInit(command CommandFactory, state StateWriter) NsInit {
	return &linuxNs{
		commandFactory: command,
		stateWriter:    state,
	}
}
