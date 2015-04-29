package journald

import (
	"fmt"

	"github.com/coreos/go-systemd/journal"
	"github.com/docker/docker/daemon/logger"
)

type Journald struct {
	Jmap map[string]string
}

func New(id string, name string) (logger.Logger, error) {
	if !journal.Enabled() {
		return nil, fmt.Errorf("journald is not enabled on this host")
	}
	// Strip a leading slash so that people can search for
	// CONTAINER_NAME=foo rather than CONTAINER_NAME=/foo.
	if name[0] == '/' {
		name = name[1:]
	}
	jmap := map[string]string{
		"CONTAINER_ID":      id[:12],
		"CONTAINER_ID_FULL": id,
		"CONTAINER_NAME":    name}
	return &Journald{Jmap: jmap}, nil
}

func (s *Journald) Log(msg *logger.Message) error {
	if msg.Source == "stderr" {
		return journal.Send(string(msg.Line), journal.PriErr, s.Jmap)
	}
	return journal.Send(string(msg.Line), journal.PriInfo, s.Jmap)
}

func (s *Journald) Close() error {
	return nil
}

func (s *Journald) Name() string {
	return "Journald"
}
