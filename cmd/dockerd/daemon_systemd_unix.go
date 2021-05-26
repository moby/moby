// +build !nosystemd

package main

import (
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/daemon/config"
	systemdDaemon "github.com/coreos/go-systemd/v22/daemon"
)

func newCgroupParent(config *config.Config) string {
	cgroupParent := "docker"
	useSystemd := daemon.UsingSystemd(config)
	if useSystemd {
		cgroupParent = "system.slice"
	}
	if config.CgroupParent != "" {
		cgroupParent = config.CgroupParent
	}
	if useSystemd {
		cgroupParent = cgroupParent + ":" + "docker" + ":"
	}
	return cgroupParent
}
