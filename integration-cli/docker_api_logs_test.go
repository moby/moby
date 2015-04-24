package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os/exec"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestLogsApiWithStdout(c *check.C) {
	name := "logs_test"

	runCmd := exec.Command(dockerBinary, "run", "-d", "-t", "--name", name, "busybox", "bin/sh", "-c", "sleep 10 && echo "+name)
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		c.Fatal(out, err)
	}

	status, body, err := sockRequest("GET", fmt.Sprintf("/containers/%s/logs?follow=1&stdout=1&timestamps=1", name), nil)
	c.Assert(status, check.Equals, http.StatusOK)
	c.Assert(err, check.IsNil)

	if !bytes.Contains(body, []byte(name)) {
		c.Fatalf("Expected %s, got %s", name, string(body[:]))
	}
}

func (s *DockerSuite) TestLogsApiNoStdoutNorStderr(c *check.C) {
	name := "logs_test"
	runCmd := exec.Command(dockerBinary, "run", "-d", "-t", "--name", name, "busybox", "/bin/sh")
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		c.Fatal(out, err)
	}

	status, body, err := sockRequest("GET", fmt.Sprintf("/containers/%s/logs", name), nil)
	c.Assert(status, check.Equals, http.StatusBadRequest)
	c.Assert(err, check.IsNil)

	expected := "Bad parameters: you must choose at least one stream"
	if !bytes.Contains(body, []byte(expected)) {
		c.Fatalf("Expected %s, got %s", expected, string(body[:]))
	}
}
