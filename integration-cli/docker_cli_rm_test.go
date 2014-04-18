package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestRemoveContainerWithRemovedVolume(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--name", "losemyvolumes", "-v", "/tmp/testing:/test", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove("/tmp/testing"); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "rm", "-v", "losemyvolumes")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()

	logDone("rm - removed volume")
}

func TestRemoveContainerWithVolume(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--name", "foo", "-v", "/srv", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "rm", "-v", "foo")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()

	logDone("rm - volume")
}

func TestRemoveContainerRunning(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-d", "--name", "foo", "busybox", "sleep", "300")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	// Test cannot remove running container
	cmd = exec.Command(dockerBinary, "rm", "foo")
	if _, err := runCommand(cmd); err == nil {
		t.Fatalf("Expected error, can't rm a running container")
	}

	// Remove with -f
	cmd = exec.Command(dockerBinary, "rm", "-f", "foo")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()

	logDone("rm - running container")
}
