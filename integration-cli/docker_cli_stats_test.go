package main

import (
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestStatsNoStream(c *check.C) {
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "busybox", "top"))
	c.Assert(err, check.IsNil)
	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), check.IsNil)

	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "stats", "--no-stream", id))
	c.Assert(err, check.IsNil)
	if !strings.Contains(out, id) {
		c.Fatalf("Expected output to contain %s, got instead: %s", id, out)
	}
}

func (*DockerSuite) TestStatsAllNoStream(c *check.C) {
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "busybox", "top"))
	c.Assert(err, check.IsNil)
	id1 := strings.TrimSpace(out)
	c.Assert(waitRun(id1), check.IsNil)

	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "busybox", "top"))
	c.Assert(err, check.IsNil)
	id2 := strings.TrimSpace(out)
	c.Assert(waitRun(id2), check.IsNil)

	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "stats", "--no-stream"))
	c.Assert(err, check.IsNil)
	if !strings.Contains(out, id1[:12]) || !strings.Contains(out, id2[:12]) {
		c.Fatalf("Expected output to contain both %s and %s, got instead: %s", id1, id2, out)
	}
}
