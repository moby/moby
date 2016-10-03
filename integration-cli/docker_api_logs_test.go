package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestLogsAPIWithStdout(c *check.C) {
	out, _ := dockerCmd(c, "run", "-d", "-t", "busybox", "/bin/sh", "-c", "while true; do echo hello; sleep 1; done")
	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	type logOut struct {
		out string
		res *http.Response
		err error
	}
	chLog := make(chan logOut)

	go func() {
		res, body, err := sockRequestRaw("GET", fmt.Sprintf("/containers/%s/logs?follow=1&stdout=1&timestamps=1", id), nil, "")
		if err != nil {
			chLog <- logOut{"", nil, err}
			return
		}
		defer body.Close()
		out, err := bufio.NewReader(body).ReadString('\n')
		if err != nil {
			chLog <- logOut{"", nil, err}
			return
		}
		chLog <- logOut{strings.TrimSpace(out), res, err}
	}()

	select {
	case l := <-chLog:
		c.Assert(l.err, checker.IsNil)
		c.Assert(l.res.StatusCode, checker.Equals, http.StatusOK)
		if !strings.HasSuffix(l.out, "hello") {
			c.Fatalf("expected log output to container 'hello', but it does not")
		}
	case <-time.After(20 * time.Second):
		c.Fatal("timeout waiting for logs to exit")
	}
}

func (s *DockerSuite) TestLogsAPINoStdoutNorStderr(c *check.C) {
	name := "logs_test"
	dockerCmd(c, "run", "-d", "-t", "--name", name, "busybox", "/bin/sh")

	status, body, err := sockRequest("GET", fmt.Sprintf("/containers/%s/logs", name), nil)
	c.Assert(status, checker.Equals, http.StatusBadRequest)
	c.Assert(err, checker.IsNil)

	expected := "Bad parameters: you must choose at least one stream"
	c.Assert(getErrorMessage(c, body), checker.Contains, expected)
}

// Regression test for #12704
func (s *DockerSuite) TestLogsAPIFollowEmptyOutput(c *check.C) {
	name := "logs_test"
	t0 := time.Now()
	dockerCmd(c, "run", "-d", "-t", "--name", name, "busybox", "sleep", "10")

	_, body, err := sockRequestRaw("GET", fmt.Sprintf("/containers/%s/logs?follow=1&stdout=1&stderr=1&tail=all", name), bytes.NewBuffer(nil), "")
	t1 := time.Now()
	c.Assert(err, checker.IsNil)
	body.Close()
	elapsed := t1.Sub(t0).Seconds()
	if elapsed > 20.0 {
		c.Fatalf("HTTP response was not immediate (elapsed %.1fs)", elapsed)
	}
}

func (s *DockerSuite) TestLogsAPIContainerNotFound(c *check.C) {
	name := "nonExistentContainer"
	resp, _, err := sockRequestRaw("GET", fmt.Sprintf("/containers/%s/logs?follow=1&stdout=1&stderr=1&tail=all", name), bytes.NewBuffer(nil), "")
	c.Assert(err, checker.IsNil)
	c.Assert(resp.StatusCode, checker.Equals, http.StatusNotFound)
}
