package nsinit

import (
	"github.com/dotcloud/docker/pkg/libcontainer"
	"log"
)

type NsInit interface {
	Exec(container *libcontainer.Container, term Terminal, args []string) (int, error)
	ExecIn(container *libcontainer.Container, nspid int, args []string) (int, error)
	Init(container *libcontainer.Container, uncleanRootfs, console string, syncPipe *SyncPipe, args []string) error
}

type linuxNs struct {
	root           string
	logFile        string
	logger         *log.Logger
	commandFactory CommandFactory
	stateWriter    StateWriter
}

func NewNsInit(logger *log.Logger, logFile string, command CommandFactory, state StateWriter) NsInit {
	return &linuxNs{
		logger:         logger,
		commandFactory: command,
		stateWriter:    state,
		logFile:        logFile,
	}
}
