package main

import (
	"os/exec"
	"strings"
	"testing"
)

// FIXME: we need a test for pulling all aliases for an image (issue #8141)

// pulling an image from the central registry should work
func TestPullImageFromCentralRegistry(t *testing.T) {
	pullCmd := exec.Command(dockerBinary, "pull", "hello-world")
	if out, _, err := runCommandWithOutput(pullCmd); err != nil {
		t.Fatalf("pulling the hello-world image from the registry has failed: %s, %v", out, err)
	}
	logDone("pull - pull hello-world")
}

// pulling a non-existing image from the central registry should return a non-zero exit code
func TestPullNonExistingImage(t *testing.T) {
	pullCmd := exec.Command(dockerBinary, "pull", "fooblahblah1234")
	if out, _, err := runCommandWithOutput(pullCmd); err == nil {
		t.Fatalf("expected non-zero exit status when pulling non-existing image: %s", out)
	}
	logDone("pull - pull fooblahblah1234 (non-existing image)")
}

// pulling an image from the central registry using official names should work
// ensure all pulls result in the same image
func TestPullImageOfficialNames(t *testing.T) {
	names := []string{
		"docker.io/hello-world",
		"index.docker.io/hello-world",
		"library/hello-world",
		"docker.io/library/hello-world",
		"index.docker.io/library/hello-world",
	}
	for _, name := range names {
		pullCmd := exec.Command(dockerBinary, "pull", name)
		out, exitCode, err := runCommandWithOutput(pullCmd)
		if err != nil || exitCode != 0 {
			t.Errorf("pulling the '%s' image from the registry has failed: %s", name, err)
			continue
		}

		// ensure we don't have multiple image names.
		imagesCmd := exec.Command(dockerBinary, "images")
		out, _, err = runCommandWithOutput(imagesCmd)
		if err != nil {
			t.Errorf("listing images failed with errors: %v", err)
		} else if strings.Contains(out, name) {
			t.Errorf("images should not have listed '%s'", name)
		}
	}
	logDone("pull - pull official names")
}
