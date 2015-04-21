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

	statusCode, body, err := sockRequest("GET", fmt.Sprintf("/containers/%s/logs?follow=1&stdout=1&timestamps=1", name), nil)

	if err != nil || statusCode != http.StatusOK {
		c.Fatalf("Expected %d from logs request, got %d", http.StatusOK, statusCode)
	}

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

	statusCode, body, err := sockRequest("GET", fmt.Sprintf("/containers/%s/logs", name), nil)

	if err == nil || statusCode != http.StatusBadRequest {
		c.Fatalf("Expected %d from logs request, got %d", http.StatusBadRequest, statusCode)
	}

	expected := "Bad parameters: you must choose at least one stream"
	if !bytes.Contains(body, []byte(expected)) {
		c.Fatalf("Expected %s, got %s", expected, string(body[:]))
	}
}
