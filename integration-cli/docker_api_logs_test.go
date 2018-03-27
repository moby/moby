package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/request"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/go-check/check"
	"golang.org/x/net/context"
)

func (s *DockerSuite) TestLogsAPIWithStdout(c *check.C) {
	out, _ := dockerCmd(c, "run", "-d", "-t", "busybox", "/bin/sh", "-c", "while true; do echo hello; sleep 1; done")
	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	type logOut struct {
		out string
		err error
	}

	chLog := make(chan logOut)
	res, body, err := request.Get(fmt.Sprintf("/containers/%s/logs?follow=1&stdout=1&timestamps=1", id))
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

	go func() {
		defer body.Close()
		out, err := bufio.NewReader(body).ReadString('\n')
		if err != nil {
			chLog <- logOut{"", err}
			return
		}
		chLog <- logOut{strings.TrimSpace(out), err}
	}()

	select {
	case l := <-chLog:
		c.Assert(l.err, checker.IsNil)
		if !strings.HasSuffix(l.out, "hello") {
			c.Fatalf("expected log output to container 'hello', but it does not")
		}
	case <-time.After(30 * time.Second):
		c.Fatal("timeout waiting for logs to exit")
	}
}

func (s *DockerSuite) TestLogsAPINoStdoutNorStderr(c *check.C) {
	name := "logs_test"
	dockerCmd(c, "run", "-d", "-t", "--name", name, "busybox", "/bin/sh")
	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	_, err = cli.ContainerLogs(context.Background(), name, types.ContainerLogsOptions{})
	expected := "Bad parameters: you must choose at least one stream"
	c.Assert(err.Error(), checker.Contains, expected)
}

// Regression test for #12704
func (s *DockerSuite) TestLogsAPIFollowEmptyOutput(c *check.C) {
	name := "logs_test"
	t0 := time.Now()
	dockerCmd(c, "run", "-d", "-t", "--name", name, "busybox", "sleep", "10")

	_, body, err := request.Get(fmt.Sprintf("/containers/%s/logs?follow=1&stdout=1&stderr=1&tail=all", name))
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
	resp, _, err := request.Get(fmt.Sprintf("/containers/%s/logs?follow=1&stdout=1&stderr=1&tail=all", name))
	c.Assert(err, checker.IsNil)
	c.Assert(resp.StatusCode, checker.Equals, http.StatusNotFound)
}

func (s *DockerSuite) TestLogsAPIUntilFutureFollow(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "logsuntilfuturefollow"
	dockerCmd(c, "run", "-d", "--name", name, "busybox", "/bin/sh", "-c", "while true; do date +%s; sleep 1; done")
	c.Assert(waitRun(name), checker.IsNil)

	untilSecs := 5
	untilDur, err := time.ParseDuration(fmt.Sprintf("%ds", untilSecs))
	c.Assert(err, checker.IsNil)
	until := daemonTime(c).Add(untilDur)

	client, err := client.NewEnvClient()
	if err != nil {
		c.Fatal(err)
	}

	cfg := types.ContainerLogsOptions{Until: until.Format(time.RFC3339Nano), Follow: true, ShowStdout: true, Timestamps: true}
	reader, err := client.ContainerLogs(context.Background(), name, cfg)
	c.Assert(err, checker.IsNil)

	type logOut struct {
		out string
		err error
	}

	chLog := make(chan logOut)

	go func() {
		bufReader := bufio.NewReader(reader)
		defer reader.Close()
		for i := 0; i < untilSecs; i++ {
			out, _, err := bufReader.ReadLine()
			if err != nil {
				if err == io.EOF {
					return
				}
				chLog <- logOut{"", err}
				return
			}

			chLog <- logOut{strings.TrimSpace(string(out)), err}
		}
	}()

	for i := 0; i < untilSecs; i++ {
		select {
		case l := <-chLog:
			c.Assert(l.err, checker.IsNil)
			i, err := strconv.ParseInt(strings.Split(l.out, " ")[1], 10, 64)
			c.Assert(err, checker.IsNil)
			c.Assert(time.Unix(i, 0).UnixNano(), checker.LessOrEqualThan, until.UnixNano())
		case <-time.After(20 * time.Second):
			c.Fatal("timeout waiting for logs to exit")
		}
	}
}

func (s *DockerSuite) TestLogsAPIUntil(c *check.C) {
	name := "logsuntil"
	dockerCmd(c, "run", "--name", name, "busybox", "/bin/sh", "-c", "for i in $(seq 1 3); do echo log$i; sleep 1; done")

	client, err := client.NewEnvClient()
	if err != nil {
		c.Fatal(err)
	}

	extractBody := func(c *check.C, cfg types.ContainerLogsOptions) []string {
		reader, err := client.ContainerLogs(context.Background(), name, cfg)
		c.Assert(err, checker.IsNil)

		actualStdout := new(bytes.Buffer)
		actualStderr := ioutil.Discard
		_, err = stdcopy.StdCopy(actualStdout, actualStderr, reader)
		c.Assert(err, checker.IsNil)

		return strings.Split(actualStdout.String(), "\n")
	}

	// Get timestamp of second log line
	allLogs := extractBody(c, types.ContainerLogsOptions{Timestamps: true, ShowStdout: true})
	c.Assert(len(allLogs), checker.GreaterOrEqualThan, 3)

	t, err := time.Parse(time.RFC3339Nano, strings.Split(allLogs[1], " ")[0])
	c.Assert(err, checker.IsNil)
	until := t.Format(time.RFC3339Nano)

	// Get logs until the timestamp of second line, i.e. first two lines
	logs := extractBody(c, types.ContainerLogsOptions{Timestamps: true, ShowStdout: true, Until: until})

	// Ensure log lines after cut-off are excluded
	logsString := strings.Join(logs, "\n")
	c.Assert(logsString, checker.Not(checker.Contains), "log3", check.Commentf("unexpected log message returned, until=%v", until))
}

func (s *DockerSuite) TestLogsAPIUntilDefaultValue(c *check.C) {
	name := "logsuntildefaultval"
	dockerCmd(c, "run", "--name", name, "busybox", "/bin/sh", "-c", "for i in $(seq 1 3); do echo log$i; done")

	client, err := client.NewEnvClient()
	if err != nil {
		c.Fatal(err)
	}

	extractBody := func(c *check.C, cfg types.ContainerLogsOptions) []string {
		reader, err := client.ContainerLogs(context.Background(), name, cfg)
		c.Assert(err, checker.IsNil)

		actualStdout := new(bytes.Buffer)
		actualStderr := ioutil.Discard
		_, err = stdcopy.StdCopy(actualStdout, actualStderr, reader)
		c.Assert(err, checker.IsNil)

		return strings.Split(actualStdout.String(), "\n")
	}

	// Get timestamp of second log line
	allLogs := extractBody(c, types.ContainerLogsOptions{Timestamps: true, ShowStdout: true})

	// Test with default value specified and parameter omitted
	defaultLogs := extractBody(c, types.ContainerLogsOptions{Timestamps: true, ShowStdout: true, Until: "0"})
	c.Assert(defaultLogs, checker.DeepEquals, allLogs)
}
