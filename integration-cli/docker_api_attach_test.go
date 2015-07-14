package main

import (
	"bytes"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/go-check/check"

	"code.google.com/p/go.net/websocket"
)

func (s *DockerSuite) TestGetContainersAttachWebsocket(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-dit", "busybox", "cat")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf(out, err)
	}

	rwc, err := sockConn(time.Duration(10 * time.Second))
	if err != nil {
		c.Fatal(err)
	}

	cleanedContainerID := strings.TrimSpace(out)
	config, err := websocket.NewConfig(
		"/containers/"+cleanedContainerID+"/attach/ws?stream=1&stdin=1&stdout=1&stderr=1",
		"http://localhost",
	)
	if err != nil {
		c.Fatal(err)
	}

	ws, err := websocket.NewClient(config, rwc)
	if err != nil {
		c.Fatal(err)
	}
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
		if err != nil {
			c.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		c.Fatal("Timeout writing to ws")
	}

	select {
	case err := <-outChan:
		if err != nil {
			c.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		c.Fatal("Timeout reading from ws")
	}

	if !bytes.Equal(expected, actual) {
		c.Fatal("Expected output on websocket to match input")
	}
}

// regression gh14320
func (s *DockerSuite) TestPostContainersAttachContainerNotFound(c *check.C) {
	status, body, err := sockRequest("POST", "/containers/doesnotexist/attach", nil)
	c.Assert(status, check.Equals, http.StatusNotFound)
	c.Assert(err, check.IsNil)
	expected := "no such id: doesnotexist\n"
	if !strings.Contains(string(body), expected) {
		c.Fatalf("Expected response body to contain %q", expected)
	}
}

func (s *DockerSuite) TestGetContainersWsAttachContainerNotFound(c *check.C) {
	status, body, err := sockRequest("GET", "/containers/doesnotexist/attach/ws", nil)
	c.Assert(status, check.Equals, http.StatusNotFound)
	c.Assert(err, check.IsNil)
	expected := "no such id: doesnotexist\n"
	if !strings.Contains(string(body), expected) {
		c.Fatalf("Expected response body to contain %q", expected)
	}
}
