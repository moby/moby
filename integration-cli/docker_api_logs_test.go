package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os/exec"
	"time"

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

// Regression test for #12704
func (s *DockerSuite) TestLogsApiFollowEmptyOutput(c *check.C) {
	defer deleteAllContainers()
	name := "logs_test"
	t0 := time.Now()
	runCmd := exec.Command(dockerBinary, "run", "-d", "-t", "--name", name, "busybox", "sleep", "10")
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		c.Fatal(out, err)
	}

	_, body, err := sockRequestRaw("GET", fmt.Sprintf("/containers/%s/logs?follow=1&stdout=1&stderr=1&tail=all", name), bytes.NewBuffer(nil), "")
	t1 := time.Now()
	body.Close()
	if err != nil {
		c.Fatal(err)
	}
	elapsed := t1.Sub(t0).Seconds()
	if elapsed > 5.0 {
		c.Fatalf("HTTP response was not immediate (elapsed %.1fs)", elapsed)
	}
}
