package main

import (
	"fmt"
	"os/exec"
	"testing"
)

// these tests need a freshly started empty private docker registry

// pulling an image from the central registry should work
func TestPushBusyboxImage(t *testing.T) {
	// skip this test until we're able to use a registry
	t.Skip()
	// tag the image to upload it tot he private registry
	repoName := fmt.Sprintf("%v/busybox", privateRegistryURL)
	tagCmd := exec.Command(dockerBinary, "tag", "busybox", repoName)
	if out, _, err := runCommandWithOutput(tagCmd); err != nil {
		t.Fatalf("image tagging failed: %s, %v", out, err)
	}

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	if out, _, err := runCommandWithOutput(pushCmd); err != nil {
		t.Fatalf("pushing the image to the private registry has failed: %s, %v", out, err)
	}

	deleteImages(repoName)

	logDone("push - push busybox to private registry")
}

// pushing an image without a prefix should throw an error
func TestPushUnprefixedRepo(t *testing.T) {
	// skip this test until we're able to use a registry
	t.Skip()
	pushCmd := exec.Command(dockerBinary, "push", "busybox")
	if out, _, err := runCommandWithOutput(pushCmd); err == nil {
		t.Fatalf("pushing an unprefixed repo didn't result in a non-zero exit status: %s", out)
	}
	logDone("push - push unprefixed busybox repo --> must fail")
}
