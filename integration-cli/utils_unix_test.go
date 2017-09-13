// +build linux freebsd solaris openbsd

package main

import (
	"github.com/docker/docker/pkg/parsers/kernel"
)

// GetKernelVersion gets the current kernel version.
func GetKernelVersion() *kernel.VersionInfo {
	v, _ := kernel.ParseRelease(testEnv.DaemonInfo.KernelVersion)
	return v
}

// CheckKernelVersion checks if current kernel is newer than (or equal to)
// the given version.
func CheckKernelVersion(k, major, minor int) bool {
	return kernel.CompareKernelVersion(*GetKernelVersion(), kernel.VersionInfo{Kernel: k, Major: major, Minor: minor}) > 0
}
