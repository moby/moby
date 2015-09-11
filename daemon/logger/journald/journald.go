// +build linux

// Package journald provides the log driver for forwarding server logs
// to endpoints that receive the systemd format.
package journald

import (
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/go-systemd/journal"
	"github.com/docker/docker/daemon/logger"
)

const name = "journald"

type journald struct {
	vars    map[string]string // additional variables and values to send to the journal along with the log message
	readers readerList
}

type readerList struct {
	mu      sync.Mutex
	readers map[*logger.LogWatcher]*logger.LogWatcher
}

func init() {
	if err := logger.RegisterLogDriver(name, New); err != nil {
		logrus.Fatal(err)
	}
	if err := logger.RegisterLogOptValidator(name, validateLogOpt); err != nil {
		logrus.Fatal(err)
	}
}

// New creates a journald logger using the configuration passed in on
// the context.
func New(ctx logger.Context) (logger.Logger, error) {
	if !journal.Enabled() {
		return nil, fmt.Errorf("journald is not enabled on this host")
	}
	// Strip a leading slash so that people can search for
	// CONTAINER_NAME=foo rather than CONTAINER_NAME=/foo.
	name := ctx.ContainerName
	if name[0] == '/' {
		name = name[1:]
	}
	vars := map[string]string{
		"CONTAINER_ID":      ctx.ContainerID[:12],
		"CONTAINER_ID_FULL": ctx.ContainerID,
		"CONTAINER_NAME":    name}
	return &journald{vars: vars, readers: readerList{readers: make(map[*logger.LogWatcher]*logger.LogWatcher)}}, nil
}

// We don't actually accept any options, but we have to supply a callback for
// the factory to pass the (probably empty) configuration map to.
func validateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		switch key {
		default:
			return fmt.Errorf("unknown log opt '%s' for journald log driver", key)
		}
	}
	return nil
}

func (s *journald) Log(msg *logger.Message) error {
	if msg.Source == "stderr" {
		return journal.Send(string(msg.Line), journal.PriErr, s.vars)
	}
	return journal.Send(string(msg.Line), journal.PriInfo, s.vars)
}

func (s *journald) Name() string {
	return name
}
