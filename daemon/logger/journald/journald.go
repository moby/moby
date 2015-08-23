// +build linux

// Package journald provides the log driver for forwarding server logs
// to endpoints that receive the systemd format.
package journald

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/go-systemd/journal"
	"github.com/docker/docker/daemon/logger"
)

const name = "journald"

type journald struct {
	Jmap map[string]string
}

func init() {
	if err := logger.RegisterLogDriver(name, New); err != nil {
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
	jmap := map[string]string{
		"CONTAINER_ID":      ctx.ContainerID[:12],
		"CONTAINER_ID_FULL": ctx.ContainerID,
		"CONTAINER_NAME":    name}
	return &journald{Jmap: jmap}, nil
}

func (s *journald) Log(msg *logger.Message) error {
	if msg.Source == "stderr" {
		return journal.Send(string(msg.Line), journal.PriErr, s.Jmap)
	}
	return journal.Send(string(msg.Line), journal.PriInfo, s.Jmap)
}

func (s *journald) Close() error {
	return nil
}

func (s *journald) Name() string {
	return name
}
