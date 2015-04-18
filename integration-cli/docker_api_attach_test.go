package main

import (
	"bytes"
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
	outChan := make(chan string)
	go func() {
		if _, err := ws.Read(actual); err != nil {
			c.Fatal(err)
		}
		outChan <- "done"
	}()

	inChan := make(chan string)
	go func() {
		if _, err := ws.Write(expected); err != nil {
			c.Fatal(err)
		}
		inChan <- "done"
	}()

	<-inChan
	<-outChan

	if !bytes.Equal(expected, actual) {
		c.Fatal("Expected output on websocket to match input")
	}
}
