// +build windows

package main

var (
	NotSystemdCgroups = TestRequirement{
		func() bool {
			return true
		},
		"Test requires cgroups are not controlled by systemd.",
	}
)
