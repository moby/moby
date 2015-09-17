package main

import (
	"bytes"
	"os/exec"
	"strings"
	"time"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestStatsNoStream(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), check.IsNil)

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
		if outerr.err != nil {
			c.Fatalf("Error running stats: %v", outerr.err)
		}
		if !bytes.Contains(outerr.out, []byte(id)) {
			c.Fatalf("running container wasn't present in output")
		}
	case <-time.After(3 * time.Second):
		statsCmd.Process.Kill()
		c.Fatalf("stats did not return immediately when not streaming")
	}
}

func (s *DockerSuite) TestStatsContainerNotFound(c *check.C) {
	testRequires(c, DaemonIsLinux)

	out, _, err := dockerCmdWithError("stats", "notfound")
	c.Assert(err, check.NotNil)
	if !strings.Contains(out, "no such id: notfound") {
		c.Fatalf("Expected to fail on not found container stats, got %q instead", out)
	}

	out, _, err = dockerCmdWithError("stats", "--no-stream", "notfound")
	c.Assert(err, check.NotNil)
	if !strings.Contains(out, "no such id: notfound") {
		c.Fatalf("Expected to fail on not found container stats with --no-stream, got %q instead", out)
	}
}
