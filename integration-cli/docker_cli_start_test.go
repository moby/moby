package main

import (
	"os/exec"
	"testing"
	"time"
)

// Regression test for https://github.com/docker/docker/issues/7843
func TestStartAttachReturnsOnError(t *testing.T) {
	defer deleteAllContainers()

	if _, err := runCommand(exec.Command(dockerBinary, "run", "-d", "--name", "test", "busybox", "/")); err == nil {
		t.Fatal("Container started even though it should not have")
	}

	ch := make(chan struct{})
	go func() {
		// Attempt to start attached to the container that won't start
		// This should return an error immediately since the container can't be started
		if _, err := runCommand(exec.Command(dockerBinary, "start", "-a", "test2")); err == nil {
			t.Fatal("Expected error but got none")
		}
		close(ch)
	}()

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatalf("Attach did not exit properly")
	}

	logDone("start - error on start with attach exits")
}
