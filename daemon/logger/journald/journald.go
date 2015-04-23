package journald

import (
	"fmt"

	"github.com/coreos/go-systemd/journal"
	"github.com/docker/docker/daemon/logger"
)

type Journald struct {
	Jmap map[string]string
}

func New(id string) (logger.Logger, error) {
	if !journal.Enabled() {
		return nil, fmt.Errorf("journald is not enabled on this host")
	}
	jmap := map[string]string{"MESSAGE_ID": id}
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
