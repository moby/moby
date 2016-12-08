// +build !windows

package main

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

type logMessage struct {
	err  error
	data []byte
}

func (s *DockerSwarmSuite) TestServiceLogs(c *check.C) {
	testRequires(c, ExperimentalDaemon)

	d := s.AddDaemon(c, true, true)

	// we have multiple services here for detecting the goroutine issue #28915
	services := map[string]string{
		"TestServiceLogs1": "hello1",
		"TestServiceLogs2": "hello2",
	}

	for name, message := range services {
		out, err := d.Cmd("service", "create", "--name", name, "busybox",
			"sh", "-c", fmt.Sprintf("echo %s; tail -f /dev/null", message))
		c.Assert(err, checker.IsNil)
		c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")
	}

	// make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout,
		d.CheckActiveContainerCount, checker.Equals, len(services))

	for name, message := range services {
		out, err := d.Cmd("service", "logs", name)
		c.Assert(err, checker.IsNil)
		c.Logf("log for %q: %q", name, out)
		c.Assert(out, checker.Contains, message)
	}
}

func (s *DockerSwarmSuite) TestServiceLogsFollow(c *check.C) {
	testRequires(c, ExperimentalDaemon)

	d := s.AddDaemon(c, true, true)

	name := "TestServiceLogsFollow"

	out, err := d.Cmd("service", "create", "--name", name, "busybox", "sh", "-c", "while true; do echo log test; sleep 0.1; done")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	// make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, 1)

	args := []string{"service", "logs", "-f", name}
	cmd := exec.Command(dockerBinary, d.PrependHostArg(args)...)
	r, w := io.Pipe()
	cmd.Stdout = w
	cmd.Stderr = w
	c.Assert(cmd.Start(), checker.IsNil)

	// Make sure pipe is written to
	ch := make(chan *logMessage)
	done := make(chan struct{})
	go func() {
		reader := bufio.NewReader(r)
		for {
			msg := &logMessage{}
			msg.data, _, msg.err = reader.ReadLine()
			select {
			case ch <- msg:
			case <-done:
				return
			}
		}
	}()

	for i := 0; i < 3; i++ {
		msg := <-ch
		c.Assert(msg.err, checker.IsNil)
		c.Assert(string(msg.data), checker.Contains, "log test")
	}
	close(done)

	c.Assert(cmd.Process.Kill(), checker.IsNil)
}
