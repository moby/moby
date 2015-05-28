// +build !test_no_exec

package main

import (
	"bytes"
	"fmt"
	"net/http"
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

	status, body, err := sockRequest("POST", fmt.Sprintf("/containers/%s/exec", name), map[string]interface{}{"Cmd": nil})
	c.Assert(status, check.Equals, http.StatusInternalServerError)
	c.Assert(err, check.IsNil)

	if !bytes.Contains(body, []byte("No exec command specified")) {
		c.Fatalf("Expected message when creating exec command with no Cmd specified")
	}
}
