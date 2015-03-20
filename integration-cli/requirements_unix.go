// +build !windows

package main

import (
	"github.com/docker/libcontainer/cgroups/systemd"
)

var (
	NotSystemdCgroups = TestRequirement{
		func() bool {
			return systemd.UseSystemd()
		},
		"Test requires cgroups are not controlled by systemd.",
	}
)
