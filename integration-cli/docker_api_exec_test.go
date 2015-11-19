// +build !test_no_exec

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-check/check"
)

// Regression test for #9414
func (s *DockerSuite) TestExecApiCreateNoCmd(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "exec_test"
	dockerCmd(c, "run", "-d", "-t", "--name", name, "busybox", "/bin/sh")

	status, body, err := sockRequest("POST", fmt.Sprintf("/containers/%s/exec", name), map[string]interface{}{"Cmd": nil})
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusInternalServerError)

	if !bytes.Contains(body, []byte("No exec command specified")) {
		c.Fatalf("Expected message when creating exec command with no Cmd specified")
	}
}

func (s *DockerSuite) TestExecApiCreateNoValidContentType(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "exec_test"
	dockerCmd(c, "run", "-d", "-t", "--name", name, "busybox", "/bin/sh")

	jsonData := bytes.NewBuffer(nil)
	if err := json.NewEncoder(jsonData).Encode(map[string]interface{}{"Cmd": nil}); err != nil {
		c.Fatalf("Can not encode data to json %s", err)
	}

	res, body, err := sockRequestRaw("POST", fmt.Sprintf("/containers/%s/exec", name), jsonData, "text/plain")
	c.Assert(err, check.IsNil)
	c.Assert(res.StatusCode, check.Equals, http.StatusInternalServerError)

	b, err := readBody(body)
	c.Assert(err, check.IsNil)

	if !bytes.Contains(b, []byte("Content-Type specified")) {
		c.Fatalf("Expected message when creating exec command with invalid Content-Type specified")
	}
}

func (s *DockerSuite) TestExecApiCreateContainerPaused(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "exec_create_test"
	dockerCmd(c, "run", "-d", "-t", "--name", name, "busybox", "/bin/sh")

	dockerCmd(c, "pause", name)
	status, body, err := sockRequest("POST", fmt.Sprintf("/containers/%s/exec", name), map[string]interface{}{"Cmd": []string{"true"}})
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusConflict)

	if !bytes.Contains(body, []byte("Container "+name+" is paused, unpause the container before exec")) {
		c.Fatalf("Expected message when creating exec command with Container %s is paused", name)
	}
}

func (s *DockerSuite) TestExecAPIStart(c *check.C) {
	dockerCmd(c, "run", "-d", "--name", "test", "busybox", "top")

	startExec := func(id string, code int) {
		resp, body, err := sockRequestRaw("POST", fmt.Sprintf("/exec/%s/start", id), strings.NewReader(`{"Detach": true}`), "application/json")
		c.Assert(err, check.IsNil)

		b, err := readBody(body)
		comment := check.Commentf("response body: %s", b)
		c.Assert(err, check.IsNil, comment)
		c.Assert(resp.StatusCode, check.Equals, code, comment)
	}

	id := createExec(c, "test")
	startExec(id, http.StatusOK)

	id = createExec(c, "test")
	dockerCmd(c, "stop", "test")

	startExec(id, http.StatusNotFound)

	dockerCmd(c, "start", "test")
	startExec(id, http.StatusNotFound)

	// make sure exec is created before pausing
	id = createExec(c, "test")
	dockerCmd(c, "pause", "test")
	startExec(id, http.StatusConflict)
	dockerCmd(c, "unpause", "test")
	startExec(id, http.StatusOK)
}

func (s *DockerSuite) TestExecAPIStartBackwardsCompatible(c *check.C) {
	dockerCmd(c, "run", "-d", "--name", "test", "busybox", "top")
	id := createExec(c, "test")

	resp, body, err := sockRequestRaw("POST", fmt.Sprintf("/v1.20/exec/%s/start", id), strings.NewReader(`{"Detach": true}`), "text/plain")
	c.Assert(err, check.IsNil)

	b, err := readBody(body)
	comment := check.Commentf("response body: %s", b)
	c.Assert(err, check.IsNil, comment)
	c.Assert(resp.StatusCode, check.Equals, http.StatusOK, comment)
}

func createExec(c *check.C, name string) string {
	_, b, err := sockRequest("POST", fmt.Sprintf("/containers/%s/exec", name), map[string]interface{}{"Cmd": []string{"true"}})
	c.Assert(err, check.IsNil, check.Commentf(string(b)))

	createResp := struct {
		ID string `json:"Id"`
	}{}
	c.Assert(json.Unmarshal(b, &createResp), check.IsNil, check.Commentf(string(b)))
	return createResp.ID
}
