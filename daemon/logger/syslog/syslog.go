package syslog

import (
	"fmt"
	"log/syslog"
	"os"
	"path"

	"github.com/docker/docker/daemon/logger"
)

type Syslog struct {
	writer *syslog.Writer
}

func New(tag string) (logger.Logger, error) {
	log, err := syslog.New(syslog.LOG_DAEMON, fmt.Sprintf("%s/%s", path.Base(os.Args[0]), tag))
	if err != nil {
		return nil, err
	}
	return &Syslog{
		writer: log,
	}, nil
}

func (s *Syslog) Log(msg *logger.Message) error {
	if msg.Source == "stderr" {
		return s.writer.Err(string(msg.Line))
	}
	return s.writer.Info(string(msg.Line))
}

func (s *Syslog) Close() error {
	return s.writer.Close()
}

func (s *Syslog) Name() string {
	return "Syslog"
}
