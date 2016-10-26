// +build !windows

package main

import (
	"bufio"
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

	name := "TestServiceLogs"

	out, err := d.Cmd("service", "create", "--name", name, "busybox", "sh", "-c", "while true; do echo log test; sleep 1; done")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	// make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 1)

	args := []string{"service", "logs", "-f", name}
	cmd := exec.Command(dockerBinary, d.prependHostArg(args)...)
	r, w := io.Pipe()
	cmd.Stdout = w
	cmd.Stderr = w
	c.Assert(cmd.Start(), checker.IsNil)

	// Make sure pipe is written to
	ch := make(chan *logMessage)
	go func() {
		reader := bufio.NewReader(r)
		msg := &logMessage{}
		msg.data, _, msg.err = reader.ReadLine()
		ch <- msg
	}()

	msg := <-ch
	c.Assert(msg.err, checker.IsNil)
	c.Assert(string(msg.data), checker.Contains, "log test")

	c.Assert(cmd.Process.Kill(), checker.IsNil)
}
