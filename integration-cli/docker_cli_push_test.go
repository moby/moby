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
	out, exitCode, err := runCommandWithOutput(tagCmd)
	errorOut(err, t, fmt.Sprintf("%v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("image tagging failed")
	}

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	out, exitCode, err = runCommandWithOutput(pushCmd)
	errorOut(err, t, fmt.Sprintf("%v %v", out, err))

	deleteImages(repoName)

	if err != nil || exitCode != 0 {
		t.Fatal("pushing the image to the private registry has failed")
	}
	logDone("push - push busybox to private registry")
}

// pushing an image without a prefix should throw an error
func TestPushUnprefixedRepo(t *testing.T) {
	// skip this test until we're able to use a registry
	t.Skip()
	pushCmd := exec.Command(dockerBinary, "push", "busybox")
	_, exitCode, err := runCommandWithOutput(pushCmd)

	if err == nil || exitCode == 0 {
		t.Fatal("pushing an unprefixed repo didn't result in a non-zero exit status")
	}
	logDone("push - push unprefixed busybox repo --> must fail")
}
