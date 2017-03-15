// +build !windows,!linux

package main

import (
	"github.com/go-check/check"
)

// start a service, and then make its task unhealthy during running
// finally, unhealthy task should be detected and killed
func (s *DockerSwarmSuite) TestServiceHealthRun(c *check.C) {
	testRequires(c, DaemonIsLinux) // busybox doesn't work on Windows

}

// start a service whose task is unhealthy at beginning
// its tasks should be blocked in starting stage, until health check is passed
func (s *DockerSwarmSuite) TestServiceHealthStart(c *check.C) {
	testRequires(c, DaemonIsLinux) // busybox doesn't work on Windows

}

// start a service whose task is unhealthy at beginning
// its tasks should be blocked in starting stage, until health check is passed
func (s *DockerSwarmSuite) TestServiceHealthUpdate(c *check.C) {
	testRequires(c, DaemonIsLinux) // busybox doesn't work on Windows

}
