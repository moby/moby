package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func (s *DockerSuite) BenchmarkLogsCLIRotateFollow(c *testing.B) {
	out, _ := dockerCmd(c, "run", "-d", "--log-opt", "max-size=1b", "--log-opt", "max-file=10", "busybox", "sh", "-c", "while true; do usleep 50000; echo hello; done")
	id := strings.TrimSpace(out)
	ch := make(chan error, 1)
	go func() {
		ch <- nil
		out, _, _ := dockerCmdWithError("logs", "-f", id)
		// if this returns at all, it's an error
		ch <- fmt.Errorf(out)
	}()

	<-ch
	select {
	case <-time.After(30 * time.Second):
		// ran for 30 seconds with no problem
		return
	case err := <-ch:
		if err != nil {
			c.Fatal(err)
		}
	}
}
