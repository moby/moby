package nsinit

import (
	"github.com/dotcloud/docker/pkg/libcontainer"
	"log"
)

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
	logger         *log.Logger
}

func NewNsInit(command CommandFactory, state StateWriter, logger *log.Logger) NsInit {
	return &linuxNs{
		commandFactory: command,
		stateWriter:    state,
		logger:         logger,
	}
}
