// +build experimental

package main

import (
	"io/ioutil"
	"os"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestStackRemove(c *check.C) {
	d := s.AddDaemon(c, true, true)

	stackArgs := append([]string{"stack", "remove", "UNKNOWN_STACK"})

	out, err := d.Cmd(stackArgs...)
	c.Assert(err, checker.IsNil)
	c.Assert(out, check.Equals, "Nothing found in stack: UNKNOWN_STACK\n")
}

func (s *DockerSwarmSuite) TestStackTasks(c *check.C) {
	d := s.AddDaemon(c, true, true)

	stackArgs := append([]string{"stack", "ps", "UNKNOWN_STACK"})

	out, err := d.Cmd(stackArgs...)
	c.Assert(err, checker.IsNil)
	c.Assert(out, check.Equals, "Nothing found in stack: UNKNOWN_STACK\n")
}

func (s *DockerSwarmSuite) TestStackServices(c *check.C) {
	d := s.AddDaemon(c, true, true)

	stackArgs := append([]string{"stack", "services", "UNKNOWN_STACK"})

	out, err := d.Cmd(stackArgs...)
	c.Assert(err, checker.IsNil)
	c.Assert(out, check.Equals, "Nothing found in stack: UNKNOWN_STACK\n")
}

// testDAB is the DAB JSON used for testing.
// TODO: Use template/text and substitute "Image" with the result of
// `docker inspect --format '{{index .RepoDigests 0}}' busybox:latest`
const testDAB = `{
    "Version": "0.1",
    "Services": {
	"srv1": {
	    "Image": "busybox@sha256:e4f93f6ed15a0cdd342f5aae387886fba0ab98af0a102da6276eaf24d6e6ade0",
	    "Command": ["top"]
	},
	"srv2": {
	    "Image": "busybox@sha256:e4f93f6ed15a0cdd342f5aae387886fba0ab98af0a102da6276eaf24d6e6ade0",
	    "Command": ["tail"],
	    "Args": ["-f", "/dev/null"]
	}
    }
}`

func (s *DockerSwarmSuite) TestStackWithDAB(c *check.C) {
	// setup
	testStackName := "test"
	testDABFileName := testStackName + ".dab"
	defer os.RemoveAll(testDABFileName)
	err := ioutil.WriteFile(testDABFileName, []byte(testDAB), 0444)
	c.Assert(err, checker.IsNil)
	d := s.AddDaemon(c, true, true)
	// deploy
	stackArgs := []string{"stack", "deploy", testStackName}
	out, err := d.Cmd(stackArgs...)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, "Loading bundle from test.dab\n")
	c.Assert(out, checker.Contains, "Creating service test_srv1\n")
	c.Assert(out, checker.Contains, "Creating service test_srv2\n")
	// ls
	stackArgs = []string{"stack", "ls"}
	out, err = d.Cmd(stackArgs...)
	c.Assert(err, checker.IsNil)
	c.Assert(out, check.Equals, "NAME  SERVICES\n"+"test  2\n")
	// rm
	stackArgs = []string{"stack", "rm", testStackName}
	out, err = d.Cmd(stackArgs...)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, "Removing service test_srv1\n")
	c.Assert(out, checker.Contains, "Removing service test_srv2\n")
	// ls (empty)
	stackArgs = []string{"stack", "ls"}
	out, err = d.Cmd(stackArgs...)
	c.Assert(err, checker.IsNil)
	c.Assert(out, check.Equals, "NAME  SERVICES\n")
}

func (s *DockerSwarmSuite) TestStackWithDABExtension(c *check.C) {
	// setup
	testStackName := "test.dab"
	testDABFileName := testStackName
	defer os.RemoveAll(testDABFileName)
	err := ioutil.WriteFile(testDABFileName, []byte(testDAB), 0444)
	c.Assert(err, checker.IsNil)
	d := s.AddDaemon(c, true, true)
	// deploy
	stackArgs := []string{"stack", "deploy", testStackName}
	out, err := d.Cmd(stackArgs...)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, "Loading bundle from test.dab\n")
	c.Assert(out, checker.Contains, "Creating service test_srv1\n")
	c.Assert(out, checker.Contains, "Creating service test_srv2\n")
}
