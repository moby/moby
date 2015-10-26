// +build !test_no_exec

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

// Regression test for #9414
func (s *DockerSuite) TestExecApiCreateNoCmd(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "exec_test"
	dockerCmd(c, "run", "-d", "-t", "--name", name, "busybox", "/bin/sh")

	status, body, err := sockRequest("POST", fmt.Sprintf("/containers/%s/exec", name), map[string]interface{}{"Cmd": nil})
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusInternalServerError)

	c.Assert(body,checker.Not(checker.Contains),[]byte("No exec command specified"),Commentf("Expected message when creating exec command with no Cmd specified"))
}

func (s *DockerSuite) TestExecApiCreateNoValidContentType(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "exec_test"
	dockerCmd(c, "run", "-d", "-t", "--name", name, "busybox", "/bin/sh")

	jsonData := bytes.NewBuffer(nil)
	c.Assert(json.NewEncoder(jsonData).Encode(map[string]interface{}{"Cmd": nil}), checker.IsNil,Commentf("Can not encode data to json"))

	res, body, err := sockRequestRaw("POST", fmt.Sprintf("/containers/%s/exec", name), jsonData, "text/plain")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, check.Equals, http.StatusInternalServerError)

	b, err := readBody(body)
	c.Assert(err, checker.IsNil)

	c.Assert(b,checker.Not(checker.Contains),[]byte("Content-Type specified"),Commentf("Expected message when creating exec command with invalid Content-Type specified"))
}

func (s *DockerSuite) TestExecAPIStart(c *check.C) {
	dockerCmd(c, "run", "-d", "--name", "test", "busybox", "top")

	createExec := func() string {
		_, b, err := sockRequest("POST", fmt.Sprintf("/containers/%s/exec", "test"), map[string]interface{}{"Cmd": []string{"true"}})
		c.Assert(err, checker.IsNil, Commentf(string(b)))

		createResp := struct {
			ID string `json:"Id"`
		}{}
		c.Assert(json.Unmarshal(b, &createResp), checker.IsNil, Commentf(string(b)))
		return createResp.ID
	}

	startExec := func(id string, code int) {
		resp, body, err := sockRequestRaw("POST", fmt.Sprintf("/exec/%s/start", id), strings.NewReader(`{"Detach": true}`), "application/json")
		c.Assert(err, checker.IsNil)

		b, err := readBody(body)
		c.Assert(err, checker.IsNil, Commentf(string(b)))
		c.Assert(resp.StatusCode, checker.Equals, code, Commentf(string(b)))
	}

	startExec(createExec(), http.StatusOK)

	id := createExec()
	dockerCmd(c, "stop", "test")

	startExec(id, http.StatusNotFound)

	dockerCmd(c, "start", "test")
	startExec(id, http.StatusNotFound)

	// make sure exec is created before pausing
	id = createExec()
	dockerCmd(c, "pause", "test")
	startExec(id, http.StatusConflict)
	dockerCmd(c, "unpause", "test")
	startExec(id, http.StatusOK)
}
