package main

import (
	"os/exec"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestStatsNoStream(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	statsCmd := exec.Command(dockerBinary, "stats", "--no-stream", id)
	type output struct {
		out []byte
		err error
	}

	ch := make(chan output)
	go func() {
		out, err := statsCmd.Output()
		ch <- output{out, err}
	}()

	select {
	case outerr := <-ch:
		c.Assert(outerr.err, checker.IsNil, check.Commentf("Error running stats: %v", outerr.err))
		c.Assert(string(outerr.out), checker.Contains, id) //running container wasn't present in output
	case <-time.After(3 * time.Second):
		statsCmd.Process.Kill()
		c.Fatalf("stats did not return immediately when not streaming")
	}
}

func (s *DockerSuite) TestStatsContainerNotFound(c *check.C) {
	testRequires(c, DaemonIsLinux)

	out, _, err := dockerCmdWithError("stats", "notfound")
	c.Assert(err, checker.NotNil)
	c.Assert(out, checker.Contains, "no such id: notfound", check.Commentf("Expected to fail on not found container stats, got %q instead", out))

	out, _, err = dockerCmdWithError("stats", "--no-stream", "notfound")
	c.Assert(err, checker.NotNil)
	c.Assert(out, checker.Contains, "no such id: notfound", check.Commentf("Expected to fail on not found container stats with --no-stream, got %q instead", out))
}
