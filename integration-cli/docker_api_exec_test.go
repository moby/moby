// +build !test_no_exec

package main

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/go-check/check"
)

// Regression test for #9414
func (s *DockerSuite) TestExecApiCreateNoCmd(c *check.C) {
	name := "exec_test"
	runCmd := exec.Command(dockerBinary, "run", "-d", "-t", "--name", name, "busybox", "/bin/sh")
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		c.Fatal(out, err)
	}

	_, body, err := sockRequest("POST", fmt.Sprintf("/containers/%s/exec", name), map[string]interface{}{"Cmd": nil})
	if err == nil || !bytes.Contains(body, []byte("No exec command specified")) {
		c.Fatalf("Expected error when creating exec command with no Cmd specified: %q", err)
	}
}
