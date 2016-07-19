// +build experimental

package main

import (
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestStackRemove(c *check.C) {
	d := s.AddDaemon(c, true, true)

	stackArgs := append([]string{"remove", "UNKNOWN_STACK"})

	out, err := d.Cmd("stack", stackArgs...)
	c.Assert(err, checker.IsNil)
	c.Assert(out, check.Equals, "Nothing found in stack: UNKNOWN_STACK\n")
}

func (s *DockerSwarmSuite) TestStackTasks(c *check.C) {
	d := s.AddDaemon(c, true, true)

	stackArgs := append([]string{"ps", "UNKNOWN_STACK"})

	out, err := d.Cmd("stack", stackArgs...)
	c.Assert(err, checker.IsNil)
	c.Assert(out, check.Equals, "Nothing found in stack: UNKNOWN_STACK\n")
}
