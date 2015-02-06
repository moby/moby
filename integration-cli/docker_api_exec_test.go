// +build !test_no_exec

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os/exec"
	"testing"

	"code.google.com/p/go.net/websocket"
)

// Regression test for #9414
func TestExecApiCreateNoCmd(t *testing.T) {
	defer deleteAllContainers()
	name := "exec_test"
	runCmd := exec.Command(dockerBinary, "run", "-d", "-t", "--name", name, "busybox", "/bin/sh")
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	body, err := sockRequest("POST", fmt.Sprintf("/containers/%s/exec", name), map[string]interface{}{"Cmd": nil})
	if err == nil || !bytes.Contains(body, []byte("No exec command specified")) {
		t.Fatalf("Expected error when creating exec command with no Cmd specified: %q", err)
	}

	logDone("exec create API - returns error when missing Cmd")
}

func TestGetContainersExecStartWebsocketInvalidExecId(t *testing.T) {
	config, err := websocket.NewConfig(
		"/exec//start/ws",
		"http://localhost",
	)
	if err != nil {
		t.Fatal(err)
	}
	rwc, err := net.Dial("unix", "/var/run/docker.sock")
	if err != nil {
		t.Fatal(err)
	}
	_, err = websocket.NewClient(config, rwc)
	if err == nil {
		t.Fatal("Expected an error for an invalid exec id")
	}

	logDone("exec start API - returns error with invalid exec id")
}

func TestGetContainersExecStartWebsocket(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-dit", "busybox", "sh")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf(out, err)
	}
	cleanedContainerID := stripTrailingCharacters(out)
	defer deleteAllContainers()

	// create the exec instance
	body, err := sockRequest(
		"POST",
		"/containers/"+cleanedContainerID+"/exec",
		map[string]interface{}{
			"AttachStdin":  true,
			"AttachStdout": true,
			"AttachStderr": true,
			"Tty":          false,
			"Cmd":          []string{"cat"},
		},
	)
	if body == nil && err != nil {
		t.Fatal(err)
	}
	setupExec := struct {
		Id string
	}{}
	if err = json.Unmarshal(body, &setupExec); err != nil {
		t.Fatal(err)
	}

	// connect to websocket endpoint
	config, err := websocket.NewConfig(
		"/exec/"+setupExec.Id+"/start/ws",
		"http://localhost",
	)
	if err != nil {
		t.Fatal(err)
	}
	rwc, err := net.Dial("unix", "/var/run/docker.sock")
	if err != nil {
		t.Fatal(err)
	}
	ws, err := websocket.NewClient(config, rwc)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()

	// test cat echo
	expected := []byte("hello")
	actual := make([]byte, len(expected))
	readChan := make(chan string)
	go func() {
		if _, err := ws.Read(actual); err != nil {
			t.Fatal(err)
		}
		readChan <- "done"
	}()

	writeChan := make(chan string)
	go func() {
		if _, err := ws.Write(expected); err != nil {
			t.Fatal(err)
		}
		writeChan <- "done"
	}()

	<-writeChan
	<-readChan

	if !bytes.Equal(expected, actual) {
		t.Fatalf("Output should be '%s', got '%s'", expected, actual)
	}

	logDone("exec start API - cat echo via websocket")
}

func TestGetContainersExecStartWebsocketTty(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-dit", "busybox", "sh")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf(out, err)
	}
	cleanedContainerID := stripTrailingCharacters(out)
	defer deleteAllContainers()

	// create the exec instance
	body, err := sockRequest(
		"POST",
		"/containers/"+cleanedContainerID+"/exec",
		map[string]interface{}{
			"AttachStdin":  true,
			"AttachStdout": true,
			"AttachStderr": true,
			"Tty":          true,
			"Cmd":          []string{"cat"},
		},
	)
	if body == nil && err != nil {
		t.Fatal(err)
	}
	setupExec := struct {
		Id string
	}{}
	if err = json.Unmarshal(body, &setupExec); err != nil {
		t.Fatal(err)
	}

	// connect to websocket endpoint
	config, err := websocket.NewConfig(
		"/exec/"+setupExec.Id+"/start/ws?tty=1",
		"http://localhost",
	)
	if err != nil {
		t.Fatal(err)
	}
	rwc, err := net.Dial("unix", "/var/run/docker.sock")
	if err != nil {
		t.Fatal(err)
	}
	ws, err := websocket.NewClient(config, rwc)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()

	// test ctrl-p-q detach
	writeChan := make(chan string)
	go func() {
		if _, err := ws.Write([]byte{16}); err != nil {
			t.Fatal(err)
		}
		if _, err := ws.Write([]byte{17}); err != nil {
			t.Fatal(err)
		}
		writeChan <- "done"
	}()
	<-writeChan

	// expect a read to fail
	readChan := make(chan string)
	go func() {
		if _, err := ws.Read(make([]byte, 1)); err != io.EOF {
			t.Fatal("Should have received io.EOF when trying to read from websocket after ctrl-p-q")
		}
		readChan <- "done"
	}()
	<-readChan

	logDone("exec start API - ctrl-p-q detach via websocket")
}
