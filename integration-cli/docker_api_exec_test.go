package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"testing"
)

// Regression test for #9414
func TestExecApiCreateNoCmd(t *testing.T) {
	defer deleteAllContainers()
	name := "exec_test"
	runCmd := exec.Command(dockerBinary, "run", "-d", "-t", "--name", name, "busybox", "/bin/sh")
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	body, err := sockRequest("POST", fmt.Sprintf("/containers/%s/exec", name), map[string]interface{}{"Cmd": nil})
	if err == nil || !bytes.Contains(body, []byte("No exec command specified")) {
		t.Fatalf("Expected error when creating exec command with no Cmd specified: %q", err)
	}

	logDone("exec create API - returns error when missing Cmd")
}
