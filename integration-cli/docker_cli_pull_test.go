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

func (s *DockerRegistryAuthHtpasswdSuite) TestPullNoCredentialsNotFound(c *check.C) {
	// we don't care about the actual image, we just want to see image not found
	// because that means v2 call returned 401 and we fell back to v1 which usually
	// gives a 404 (in this case the test registry doesn't handle v1 at all)
	out, _, err := dockerCmdWithError("pull", privateRegistryURL+"/busybox")
	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "Error: image busybox not found")
}
