package main

import (
	"fmt"
	"os/exec"
	"testing"
)

// these tests need a freshly started empty private docker registry

// pulling an image from the central registry should work
func TestPushBusyboxImage(t *testing.T) {
	reg, err := newTestRegistryV2(t)
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()
	repoName := fmt.Sprintf("%v/dockercli/busybox", reg.URL)
	// tag the image to upload it tot he private registry
	tagCmd := exec.Command(dockerBinary, "tag", "busybox", repoName)
	if out, _, err := runCommandWithOutput(tagCmd); err != nil {
		t.Fatalf("image tagging failed: %s, %v", out, err)
	}
	defer deleteImages(repoName)
	pushCmd := exec.Command(dockerBinary, "push", repoName)
	if out, _, err := runCommandWithOutput(pushCmd); err != nil {
		t.Fatalf("pushing the image to the private registry has failed: %s, %v", out, err)
	}
	logDone("push - push busybox to private registry")
}

// pushing an image without a prefix should throw an error
func TestPushUnprefixedRepo(t *testing.T) {
	pushCmd := exec.Command(dockerBinary, "push", "busybox")
	if out, _, err := runCommandWithOutput(pushCmd); err == nil {
		t.Fatalf("pushing an unprefixed repo didn't result in a non-zero exit status: %s", out)
	}
	logDone("push - push unprefixed busybox repo --> must fail")
}
