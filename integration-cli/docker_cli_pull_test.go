package main

import (
	"os/exec"
	"testing"
)

// FIXME: we need a test for pulling all aliases for an image (issue #8141)

// pulling an image from the central registry should work
func TestPullImageFromCentralRegistry(t *testing.T) {
	pullCmd := exec.Command(dockerBinary, "pull", "scratch")
	if out, _, err := runCommandWithOutput(pullCmd); err != nil {
		t.Fatalf("pulling the scratch image from the registry has failed: %s, %v", out, err)
	}
	logDone("pull - pull scratch")
}

// pulling a non-existing image from the central registry should return a non-zero exit code
func TestPullNonExistingImage(t *testing.T) {
	pullCmd := exec.Command(dockerBinary, "pull", "fooblahblah1234")
	if out, _, err := runCommandWithOutput(pullCmd); err == nil {
		t.Fatalf("expected non-zero exit status when pulling non-existing image: %s", out)
	}
	logDone("pull - pull fooblahblah1234 (non-existing image)")
}
