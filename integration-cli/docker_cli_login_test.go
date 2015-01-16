package main

import (
	"bytes"
	"os/exec"
	"testing"
)

func TestLoginWithoutTTY(t *testing.T) {
	cmd := exec.Command(dockerBinary, "login")

	// Send to stdin so the process does not get the TTY
	cmd.Stdin = bytes.NewBufferString("buffer test string \n")

	// run the command and block until it's done
	if err := cmd.Run(); err == nil {
		t.Fatal("Expected non nil err when loginning in & TTY not available")
	}

	logDone("login - login without TTY")
}
