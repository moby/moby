package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestStopContainerAlreadyStopped(c *check.C) {
	testRequires(c, DaemonIsLinux)

	name := "already-stopped"
	dockerCmd(c, "run", "-d", "--name", name, "busybox", "true")
	// just making sure `true` finished
	time.Sleep(3 * time.Second)
	out, _, err := dockerCmdWithError("stop", name)
	c.Assert(err, check.NotNil)
	expected := fmt.Sprintf("Conflict: container %s already stopped", name)
	if !strings.Contains(out, expected) {
		c.Fatalf("Expected %s, got %s", expected, out)
	}
}
