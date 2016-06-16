// +build !experimental !linux

package main

import (
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/registry"
)

func pluginInit(config *daemon.Config, remote libcontainerd.Remote, rs registry.Service) error {
	return nil
}
