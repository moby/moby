package main

import (
	"bytes"
	"os/exec"
	"testing"
	"time"

	"code.google.com/p/go.net/websocket"
)

func TestGetContainersAttachWebsocket(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-dit", "busybox", "cat")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf(out, err)
	}
	defer deleteAllContainers()

	rwc, err := sockConn(time.Duration(10 * time.Second))
	if err != nil {
		t.Fatal(err)
	}

	cleanedContainerID := stripTrailingCharacters(out)
	config, err := websocket.NewConfig(
		"/containers/"+cleanedContainerID+"/attach/ws?stream=1&stdin=1&stdout=1&stderr=1",
		"http://localhost",
	)
	if err != nil {
		t.Fatal(err)
	}

	ws, err := websocket.NewClient(config, rwc)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()

	expected := []byte("hello")
	actual := make([]byte, len(expected))
	outChan := make(chan string)
	go func() {
		if _, err := ws.Read(actual); err != nil {
			t.Fatal(err)
		}
		outChan <- "done"
	}()

	inChan := make(chan string)
	go func() {
		if _, err := ws.Write(expected); err != nil {
			t.Fatal(err)
		}
		inChan <- "done"
	}()

	<-inChan
	<-outChan

	if !bytes.Equal(expected, actual) {
		t.Fatal("Expected output on websocket to match input")
	}

	logDone("container attach websocket - can echo input via cat")
}
