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
