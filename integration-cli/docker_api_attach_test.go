package main

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
	"golang.org/x/net/websocket"
)

func (s *DockerSuite) TestGetContainersAttachWebsocket(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-dit", "busybox", "cat")

	rwc, err := sockConn(time.Duration(10 * time.Second))
	c.Assert(err, checker.IsNil)

	cleanedContainerID := strings.TrimSpace(out)
	config, err := websocket.NewConfig(
		"/containers/"+cleanedContainerID+"/attach/ws?stream=1&stdin=1&stdout=1&stderr=1",
		"http://localhost",
	)
	c.Assert(err, checker.IsNil)

	ws, err := websocket.NewClient(config, rwc)
	c.Assert(err, checker.IsNil)
	defer ws.Close()

	expected := []byte("hello")
	actual := make([]byte, len(expected))

	outChan := make(chan error)
	go func() {
		_, err := ws.Read(actual)
		outChan <- err
		close(outChan)
	}()

	inChan := make(chan error)
	go func() {
		_, err := ws.Write(expected)
		inChan <- err
		close(inChan)
	}()

	select {
	case err := <-inChan:
		c.Assert(err, checker.IsNil)
	case <-time.After(5 * time.Second):
		c.Fatal("Timeout writing to ws")
	}

	select {
	case err := <-outChan:
		c.Assert(err, checker.IsNil)
	case <-time.After(5 * time.Second):
		c.Fatal("Timeout reading from ws")
	}

	c.Assert(actual, checker.DeepEquals, expected, check.Commentf("Websocket didn't return the expected data"))
}

// regression gh14320
func (s *DockerSuite) TestPostContainersAttachContainerNotFound(c *check.C) {
	status, body, err := sockRequest("POST", "/containers/doesnotexist/attach", nil)
	c.Assert(status, checker.Equals, http.StatusNotFound)
	c.Assert(err, checker.IsNil)
	expected := "no such id: doesnotexist\n"
	c.Assert(string(body), checker.Contains, expected)
}

func (s *DockerSuite) TestGetContainersWsAttachContainerNotFound(c *check.C) {
	status, body, err := sockRequest("GET", "/containers/doesnotexist/attach/ws", nil)
	c.Assert(status, checker.Equals, http.StatusNotFound)
	c.Assert(err, checker.IsNil)
	expected := "no such id: doesnotexist\n"
	c.Assert(string(body), checker.Contains, expected)
}

func (s *DockerSuite) TestPostContainersAttach(c *check.C) {
	testRequires(c, DaemonIsLinux)

	expectSuccess := func(conn net.Conn, br *bufio.Reader, stream string, tty bool) {
		defer conn.Close()
		expected := []byte("success")
		_, err := conn.Write(expected)
		c.Assert(err, checker.IsNil)

		conn.SetReadDeadline(time.Now().Add(time.Second))
		lenHeader := 0
		if !tty {
			lenHeader = 8
		}
		actual := make([]byte, len(expected)+lenHeader)
		_, err = io.ReadFull(br, actual)
		c.Assert(err, checker.IsNil)
		if !tty {
			fdMap := map[string]byte{
				"stdin":  0,
				"stdout": 1,
				"stderr": 2,
			}
			c.Assert(actual[0], checker.Equals, fdMap[stream])
		}
		c.Assert(actual[lenHeader:], checker.DeepEquals, expected, check.Commentf("Attach didn't return the expected data from %s", stream))
	}

	expectTimeout := func(conn net.Conn, br *bufio.Reader, stream string) {
		defer conn.Close()
		_, err := conn.Write([]byte{'t'})
		c.Assert(err, checker.IsNil)

		conn.SetReadDeadline(time.Now().Add(time.Second))
		actual := make([]byte, 1)
		_, err = io.ReadFull(br, actual)
		opErr, ok := err.(*net.OpError)
		c.Assert(ok, checker.Equals, true, check.Commentf("Error is expected to be *net.OpError, got %v", err))
		c.Assert(opErr.Timeout(), checker.Equals, true, check.Commentf("Read from %s is expected to timeout", stream))
	}

	// Create a container that only emits stdout.
	cid, _ := dockerCmd(c, "run", "-di", "busybox", "cat")
	cid = strings.TrimSpace(cid)
	// Attach to the container's stdout stream.
	conn, br, err := sockRequestHijack("POST", "/containers/"+cid+"/attach?stream=1&stdin=1&stdout=1", nil, "text/plain")
	c.Assert(err, checker.IsNil)
	// Check if the data from stdout can be received.
	expectSuccess(conn, br, "stdout", false)
	// Attach to the container's stderr stream.
	conn, br, err = sockRequestHijack("POST", "/containers/"+cid+"/attach?stream=1&stdin=1&stderr=1", nil, "text/plain")
	c.Assert(err, checker.IsNil)
	// Since the container only emits stdout, attaching to stderr should return nothing.
	expectTimeout(conn, br, "stdout")

	// Test the simlar functions of the stderr stream.
	cid, _ = dockerCmd(c, "run", "-di", "busybox", "/bin/sh", "-c", "cat >&2")
	cid = strings.TrimSpace(cid)
	conn, br, err = sockRequestHijack("POST", "/containers/"+cid+"/attach?stream=1&stdin=1&stderr=1", nil, "text/plain")
	c.Assert(err, checker.IsNil)
	expectSuccess(conn, br, "stderr", false)
	conn, br, err = sockRequestHijack("POST", "/containers/"+cid+"/attach?stream=1&stdin=1&stdout=1", nil, "text/plain")
	c.Assert(err, checker.IsNil)
	expectTimeout(conn, br, "stderr")

	// Test with tty.
	cid, _ = dockerCmd(c, "run", "-dit", "busybox", "/bin/sh", "-c", "cat >&2")
	cid = strings.TrimSpace(cid)
	// Attach to stdout only.
	conn, br, err = sockRequestHijack("POST", "/containers/"+cid+"/attach?stream=1&stdin=1&stdout=1", nil, "text/plain")
	c.Assert(err, checker.IsNil)
	expectSuccess(conn, br, "stdout", true)

	// Attach without stdout stream.
	conn, br, err = sockRequestHijack("POST", "/containers/"+cid+"/attach?stream=1&stdin=1&stderr=1", nil, "text/plain")
	c.Assert(err, checker.IsNil)
	// Nothing should be received because both the stdout and stderr of the container will be
	// sent to the client as stdout when tty is enabled.
	expectTimeout(conn, br, "stdout")
}
