package main

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Regression test for https://github.com/docker/docker/issues/7843
func TestStartAttachReturnsOnError(t *testing.T) {
	defer deleteAllContainers()

	cmd(t, "run", "-d", "--name", "test", "busybox")
	cmd(t, "stop", "test")

	// Expect this to fail because the above container is stopped, this is what we want
	if _, err := runCommand(exec.Command(dockerBinary, "run", "-d", "--name", "test2", "--link", "test:test", "busybox")); err == nil {
		t.Fatal("Expected error but got none")
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

// gh#8726: a failed Start() breaks --volumes-from on subsequent Start()'s
func TestStartVolumesFromFailsCleanly(t *testing.T) {
	defer deleteAllContainers()

	// Create the first data volume
	cmd(t, "run", "-d", "--name", "data_before", "-v", "/foo", "busybox")

	// Expect this to fail because the data test after contaienr doesn't exist yet
	if _, err := runCommand(exec.Command(dockerBinary, "run", "-d", "--name", "consumer", "--volumes-from", "data_before", "--volumes-from", "data_after", "busybox")); err == nil {
		t.Fatal("Expected error but got none")
	}

	// Create the second data volume
	cmd(t, "run", "-d", "--name", "data_after", "-v", "/bar", "busybox")

	// Now, all the volumes should be there
	cmd(t, "start", "consumer")

	// Check that we have the volumes we want
	out, _, _ := cmd(t, "inspect", "--format='{{ len .Volumes }}'", "consumer")
	n_volumes := strings.Trim(out, " \r\n'")
	if n_volumes != "2" {
		t.Fatalf("Missing volumes: expected 2, got %s", n_volumes)
	}

	logDone("start - missing containers in --volumes-from did not affect subsequent runs")
}
