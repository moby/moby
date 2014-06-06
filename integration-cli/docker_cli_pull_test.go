package main

import (
	"fmt"
	"os/exec"
	"testing"
)

// pulling an image from the central registry should work
func TestPullImageFromCentralRegistry(t *testing.T) {
	pullCmd := exec.Command(dockerBinary, "pull", "busybox:latest")
	out, exitCode, err := runCommandWithOutput(pullCmd)
	errorOut(err, t, fmt.Sprintf("%s %s", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("pulling the busybox image from the registry has failed")
	}
	logDone("pull - pull busybox")
}

// pulling a non-existing image from the central registry should return a non-zero exit code
func TestPullNonExistingImage(t *testing.T) {
	pullCmd := exec.Command(dockerBinary, "pull", "fooblahblah1234")
	_, exitCode, err := runCommandWithOutput(pullCmd)

	if err == nil || exitCode == 0 {
		t.Fatal("expected non-zero exit status when pulling non-existing image")
	}
	logDone("pull - pull fooblahblah1234 (non-existing image)")
}
