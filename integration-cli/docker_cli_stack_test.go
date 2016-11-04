package main

import (
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestStackRemove(c *check.C) {
	testRequires(c, ExperimentalDaemon)
	d := s.AddDaemon(c, true, true)

	stackArgs := append([]string{"stack", "remove", "UNKNOWN_STACK"})

	out, err := d.Cmd(stackArgs...)
	c.Assert(err, checker.IsNil)
	c.Assert(out, check.Equals, "Nothing found in stack: UNKNOWN_STACK\n")
}

func (s *DockerSwarmSuite) TestStackTasks(c *check.C) {
	testRequires(c, ExperimentalDaemon)
	d := s.AddDaemon(c, true, true)

	stackArgs := append([]string{"stack", "ps", "UNKNOWN_STACK"})

	out, err := d.Cmd(stackArgs...)
	c.Assert(err, checker.IsNil)
	c.Assert(out, check.Equals, "Nothing found in stack: UNKNOWN_STACK\n")
}

func (s *DockerSwarmSuite) TestStackServices(c *check.C) {
	testRequires(c, ExperimentalDaemon)
	d := s.AddDaemon(c, true, true)

	stackArgs := append([]string{"stack", "services", "UNKNOWN_STACK"})

	out, err := d.Cmd(stackArgs...)
	c.Assert(err, checker.IsNil)
	c.Assert(out, check.Equals, "Nothing found in stack: UNKNOWN_STACK\n")
}

func (s *DockerSwarmSuite) TestStackDeployComposeFile(c *check.C) {
	testRequires(c, ExperimentalDaemon)
	d := s.AddDaemon(c, true, true)

	testStackName := "testdeploy"
	stackArgs := []string{
		"stack", "deploy",
		"--compose-file", "fixtures/deploy/default.yaml",
		testStackName,
	}
	out, err := d.Cmd(stackArgs...)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = d.Cmd([]string{"stack", "ls"}...)
	c.Assert(err, checker.IsNil)
	c.Assert(out, check.Equals, "NAME        SERVICES\n"+"testdeploy  2\n")

	out, err = d.Cmd([]string{"stack", "rm", testStackName}...)
	c.Assert(err, checker.IsNil)
	out, err = d.Cmd([]string{"stack", "ls"}...)
	c.Assert(err, checker.IsNil)
	c.Assert(out, check.Equals, "NAME  SERVICES\n")
}
