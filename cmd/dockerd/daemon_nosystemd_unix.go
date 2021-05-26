//go:build no_systemd

package main

import (
	"github.com/docker/docker/daemon/config"
)

func newCgroupParent(config *config.Config) string {
	if config.CgroupParent == "" {
		return "docker"
	} else {
		return config.CgroupParent
	}
}
