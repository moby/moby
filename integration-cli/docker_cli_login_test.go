package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"testing"
)

func TestLoginWithoutTTY(t *testing.T) {
	cmd := exec.Command(dockerBinary, "login")
	// setup STDOUT and STDERR so that we see any output and errors in our console
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// create a buffer with text then a new line as a return
	buf := bytes.NewBuffer([]byte("buffer test string \n"))

	// use a pipe for stdin and manually copy the data so that
	// the process does not get the TTY
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	// copy the bytes into the commands stdin along with a new line
	go io.Copy(in, buf)

	// run the command and block until it's done
	if err := cmd.Run(); err == nil {
		t.Fatal("Expected non nil err when loginning in & TTY not available")
	}

	logDone("login - login without TTY")
}
