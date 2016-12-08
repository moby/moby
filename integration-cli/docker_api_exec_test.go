// +build !test_no_exec

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

// Regression test for #9414
func (s *DockerSuite) TestExecAPICreateNoCmd(c *check.C) {
	name := "exec_test"
	dockerCmd(c, "run", "-d", "-t", "--name", name, "busybox", "/bin/sh")

	status, body, err := sockRequest("POST", fmt.Sprintf("/containers/%s/exec", name), map[string]interface{}{"Cmd": nil})
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusInternalServerError)

	comment := check.Commentf("Expected message when creating exec command with no Cmd specified")
	c.Assert(getErrorMessage(c, body), checker.Contains, "No exec command specified", comment)
}

func (s *DockerSuite) TestExecAPICreateNoValidContentType(c *check.C) {
	name := "exec_test"
	dockerCmd(c, "run", "-d", "-t", "--name", name, "busybox", "/bin/sh")

	jsonData := bytes.NewBuffer(nil)
	if err := json.NewEncoder(jsonData).Encode(map[string]interface{}{"Cmd": nil}); err != nil {
		c.Fatalf("Can not encode data to json %s", err)
	}

	res, body, err := sockRequestRaw("POST", fmt.Sprintf("/containers/%s/exec", name), jsonData, "text/plain")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusInternalServerError)

	b, err := readBody(body)
	c.Assert(err, checker.IsNil)

	comment := check.Commentf("Expected message when creating exec command with invalid Content-Type specified")
	c.Assert(getErrorMessage(c, b), checker.Contains, "Content-Type specified", comment)
}

func (s *DockerSuite) TestExecAPICreateContainerPaused(c *check.C) {
	// Not relevant on Windows as Windows containers cannot be paused
	testRequires(c, DaemonIsLinux)
	name := "exec_create_test"
	dockerCmd(c, "run", "-d", "-t", "--name", name, "busybox", "/bin/sh")

	dockerCmd(c, "pause", name)
	status, body, err := sockRequest("POST", fmt.Sprintf("/containers/%s/exec", name), map[string]interface{}{"Cmd": []string{"true"}})
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusConflict)

	comment := check.Commentf("Expected message when creating exec command with Container %s is paused", name)
	c.Assert(getErrorMessage(c, body), checker.Contains, "Container "+name+" is paused, unpause the container before exec", comment)
}

func (s *DockerSuite) TestExecAPIStart(c *check.C) {
	testRequires(c, DaemonIsLinux) // Uses pause/unpause but bits may be salvagable to Windows to Windows CI
	dockerCmd(c, "run", "-d", "--name", "test", "busybox", "top")

	id := createExec(c, "test")
	startExec(c, id, http.StatusOK)

	var execJSON struct{ PID int }
	inspectExec(c, id, &execJSON)
	c.Assert(execJSON.PID, checker.GreaterThan, 1)

	id = createExec(c, "test")
	dockerCmd(c, "stop", "test")

	startExec(c, id, http.StatusNotFound)

	dockerCmd(c, "start", "test")
	startExec(c, id, http.StatusNotFound)

	// make sure exec is created before pausing
	id = createExec(c, "test")
	dockerCmd(c, "pause", "test")
	startExec(c, id, http.StatusConflict)
	dockerCmd(c, "unpause", "test")
	startExec(c, id, http.StatusOK)
}

func (s *DockerSuite) TestExecAPIStartEnsureHeaders(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "--name", "test", "busybox", "top")

	id := createExec(c, "test")
	resp, _, err := sockRequestRaw("POST", fmt.Sprintf("/exec/%s/start", id), strings.NewReader(`{"Detach": true}`), "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(resp.Header.Get("Server"), checker.Not(checker.Equals), "")
}

func (s *DockerSuite) TestExecAPIStartBackwardsCompatible(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows only supports 1.25 or later
	runSleepingContainer(c, "-d", "--name", "test")
	id := createExec(c, "test")

	resp, body, err := sockRequestRaw("POST", fmt.Sprintf("/v1.20/exec/%s/start", id), strings.NewReader(`{"Detach": true}`), "text/plain")
	c.Assert(err, checker.IsNil)

	b, err := readBody(body)
	comment := check.Commentf("response body: %s", b)
	c.Assert(err, checker.IsNil, comment)
	c.Assert(resp.StatusCode, checker.Equals, http.StatusOK, comment)
}

// #19362
func (s *DockerSuite) TestExecAPIStartMultipleTimesError(c *check.C) {
	runSleepingContainer(c, "-d", "--name", "test")
	execID := createExec(c, "test")
	startExec(c, execID, http.StatusOK)

	timeout := time.After(60 * time.Second)
	var execJSON struct{ Running bool }
	for {
		select {
		case <-timeout:
			c.Fatal("timeout waiting for exec to start")
		default:
		}

		inspectExec(c, execID, &execJSON)
		if !execJSON.Running {
			break
		}
	}

	startExec(c, execID, http.StatusConflict)
}

// #20638
func (s *DockerSuite) TestExecAPIStartWithDetach(c *check.C) {
	name := "foo"
	runSleepingContainer(c, "-d", "-t", "--name", name)
	data := map[string]interface{}{
		"cmd":         []string{"true"},
		"AttachStdin": true,
	}
	_, b, err := sockRequest("POST", fmt.Sprintf("/containers/%s/exec", name), data)
	c.Assert(err, checker.IsNil, check.Commentf(string(b)))

	createResp := struct {
		ID string `json:"Id"`
	}{}
	c.Assert(json.Unmarshal(b, &createResp), checker.IsNil, check.Commentf(string(b)))

	_, body, err := sockRequestRaw("POST", fmt.Sprintf("/exec/%s/start", createResp.ID), strings.NewReader(`{"Detach": true}`), "application/json")
	c.Assert(err, checker.IsNil)

	b, err = readBody(body)
	comment := check.Commentf("response body: %s", b)
	c.Assert(err, checker.IsNil, comment)

	resp, _, err := sockRequestRaw("GET", "/_ping", nil, "")
	c.Assert(err, checker.IsNil)
	if resp.StatusCode != http.StatusOK {
		c.Fatal("daemon is down, it should alive")
	}
}

func createExec(c *check.C, name string) string {
	_, b, err := sockRequest("POST", fmt.Sprintf("/containers/%s/exec", name), map[string]interface{}{"Cmd": []string{"true"}})
	c.Assert(err, checker.IsNil, check.Commentf(string(b)))

	createResp := struct {
		ID string `json:"Id"`
	}{}
	c.Assert(json.Unmarshal(b, &createResp), checker.IsNil, check.Commentf(string(b)))
	return createResp.ID
}

func startExec(c *check.C, id string, code int) {
	resp, body, err := sockRequestRaw("POST", fmt.Sprintf("/exec/%s/start", id), strings.NewReader(`{"Detach": true}`), "application/json")
	c.Assert(err, checker.IsNil)

	b, err := readBody(body)
	comment := check.Commentf("response body: %s", b)
	c.Assert(err, checker.IsNil, comment)
	c.Assert(resp.StatusCode, checker.Equals, code, comment)
}

func inspectExec(c *check.C, id string, out interface{}) {
	resp, body, err := sockRequestRaw("GET", fmt.Sprintf("/exec/%s/json", id), nil, "")
	c.Assert(err, checker.IsNil)
	defer body.Close()
	c.Assert(resp.StatusCode, checker.Equals, http.StatusOK)
	err = json.NewDecoder(body).Decode(out)
	c.Assert(err, checker.IsNil)
}
