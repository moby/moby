package syslog

import (
	"fmt"
	"log/syslog"
	"os"
	"path"
	"sync"

	"github.com/docker/docker/daemon/logger"
)

type Syslog struct {
	writer *syslog.Writer
	tag    string
	mu     sync.Mutex
}

func New(tag string) (logger.Logger, error) {
	log, err := syslog.New(syslog.LOG_USER, path.Base(os.Args[0]))
	if err != nil {
		return nil, err
	}
	return &Syslog{
		writer: log,
		tag:    tag,
	}, nil
}

func (s *Syslog) Log(msg *logger.Message) error {
	logMessage := fmt.Sprintf("%s: %s", s.tag, string(msg.Line))
	if msg.Source == "stderr" {
		return s.writer.Err(logMessage)
	}
	return s.writer.Info(logMessage)
}

func (s *Syslog) Close() error {
	return s.writer.Close()
}

func (s *Syslog) Name() string {
	return "Syslog"
}
