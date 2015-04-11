package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os/exec"
	"testing"
)

func TestLogsApiWithStdout(t *testing.T) {
	defer deleteAllContainers()
	name := "logs_test"

	runCmd := exec.Command(dockerBinary, "run", "-d", "-t", "--name", name, "busybox", "bin/sh", "-c", "sleep 10 && echo "+name)
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	statusCode, body, err := sockRequest("GET", fmt.Sprintf("/containers/%s/logs?follow=1&stdout=1&timestamps=1", name), nil)

	if err != nil || statusCode != http.StatusOK {
		t.Fatalf("Expected %d from logs request, got %d", http.StatusOK, statusCode)
	}

	if !bytes.Contains(body, []byte(name)) {
		t.Fatalf("Expected %s, got %s", name, string(body[:]))
	}

	logDone("logs API - with stdout ok")
}

func TestLogsApiNoStdoutNorStderr(t *testing.T) {
	defer deleteAllContainers()
	name := "logs_test"
	runCmd := exec.Command(dockerBinary, "run", "-d", "-t", "--name", name, "busybox", "/bin/sh")
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	statusCode, body, err := sockRequest("GET", fmt.Sprintf("/containers/%s/logs", name), nil)

	if err == nil || statusCode != http.StatusBadRequest {
		t.Fatalf("Expected %d from logs request, got %d", http.StatusBadRequest, statusCode)
	}

	expected := "Bad parameters: you must choose at least one stream"
	if !bytes.Contains(body, []byte(expected)) {
		t.Fatalf("Expected %s, got %s", expected, string(body[:]))
	}

	logDone("logs API - returns error when no stdout nor stderr specified")
}
