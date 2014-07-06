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

func TestRemoveRunningContainer(t *testing.T) {
	createRunningContainer(t, "foo")

	// Test cannot remove running container
	cmd := exec.Command(dockerBinary, "rm", "foo")
	if _, err := runCommand(cmd); err == nil {
		t.Fatalf("Expected error, can't rm a running container")
	}

	deleteAllContainers()

	logDone("rm - running container")
}

func TestStopAndRemoveRunningContainer(t *testing.T) {
	createRunningContainer(t, "foo")

	// Stop then remove with -s
	cmd := exec.Command(dockerBinary, "rm", "-s", "foo")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()

	logDone("rm - running container with --stop=true")
}

func TestKillAndRemoveRunningContainer(t *testing.T) {
	createRunningContainer(t, "foo")

	// Kill then remove with -k
	cmd := exec.Command(dockerBinary, "rm", "-k", "foo")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()

	logDone("rm - running container with --kill=true")
}

func TestRemoveContainerWithStopAndKill(t *testing.T) {
	cmd := exec.Command(dockerBinary, "rm", "-sk", "foo")
	if _, err := runCommand(cmd); err == nil {
		t.Fatalf("Expected error: can't use stop and kill simulteanously")
	}
	logDone("rm - with --stop=true and --kill=true")
}

func createRunningContainer(t *testing.T, name string) {
	cmd := exec.Command(dockerBinary, "run", "-dt", "--name", name, "busybox", "top")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}
}
