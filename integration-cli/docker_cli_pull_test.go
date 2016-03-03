package main

import (
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

// TestPullClientDisconnect kills the client during a pull operation and verifies that the operation
// gets cancelled.
//
// Ref: docker/docker#15589
func (s *DockerHubPullSuite) TestPullClientDisconnect(c *check.C) {
	testRequires(c, DaemonIsLinux)
	repoName := "hello-world:latest"

	pullCmd := s.MakeCmd("pull", repoName)
	stdout, err := pullCmd.StdoutPipe()
	c.Assert(err, checker.IsNil)
	err = pullCmd.Start()
	c.Assert(err, checker.IsNil)

	// Cancel as soon as we get some output.
	buf := make([]byte, 10)
	_, err = stdout.Read(buf)
	c.Assert(err, checker.IsNil)

	err = pullCmd.Process.Kill()
	c.Assert(err, checker.IsNil)

	time.Sleep(2 * time.Second)
	_, err = s.CmdWithError("inspect", repoName)
	c.Assert(err, checker.NotNil, check.Commentf("image was pulled after client disconnected"))
}
