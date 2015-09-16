package main

import (
	"bytes"
	"os/exec"
	"strings"
	"time"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestCliStatsNoStream(c *check.C) {
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
