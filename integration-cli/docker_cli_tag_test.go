package main

import (
	"fmt"
	"os/exec"
	"testing"
)

// tagging a named image in a new unprefixed repo should work
func TestTagUnprefixedRepoByName(t *testing.T) {
	pullCmd := exec.Command(dockerBinary, "pull", "busybox")
	out, exitCode, err := runCommandWithOutput(pullCmd)
	errorOut(err, t, fmt.Sprintf("%s %s", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("pulling the busybox image from the registry has failed")
	}

	tagCmd := exec.Command(dockerBinary, "tag", "busybox", "testfoobarbaz")
	out, _, err = runCommandWithOutput(tagCmd)
	errorOut(err, t, fmt.Sprintf("%v %v", out, err))

	deleteImages("testfoobarbaz")

	logDone("tag - busybox -> testfoobarbaz")
}

// tagging an image by ID in a new unprefixed repo should work
func TestTagUnprefixedRepoByID(t *testing.T) {
	getIDCmd := exec.Command(dockerBinary, "inspect", "-f", "{{.id}}", "busybox")
	out, _, err := runCommandWithOutput(getIDCmd)
	errorOut(err, t, fmt.Sprintf("failed to get the image ID of busybox: %v", err))

	cleanedImageID := stripTrailingCharacters(out)
	tagCmd := exec.Command(dockerBinary, "tag", cleanedImageID, "testfoobarbaz")
	out, _, err = runCommandWithOutput(tagCmd)
	errorOut(err, t, fmt.Sprintf("%s %s", out, err))

	deleteImages("testfoobarbaz")

	logDone("tag - busybox's image ID -> testfoobarbaz")
}

// ensure we don't allow the use of invalid tags; these tag operations should fail
func TestTagInvalidUnprefixedRepo(t *testing.T) {
	// skip this until we start blocking bad tags
	t.Skip()

	invalidRepos := []string{"-foo", "fo$z$", "Foo@3cc", "Foo$3", "Foo*3", "Fo^3", "Foo!3", "F)xcz(", "fo", "f"}

	for _, repo := range invalidRepos {
		tagCmd := exec.Command(dockerBinary, "tag", "busybox", repo)
		_, _, err := runCommandWithOutput(tagCmd)
		if err == nil {
			t.Errorf("tag busybox %v should have failed", repo)
			continue
		}
		logMessage := fmt.Sprintf("tag - busybox %v --> must fail", repo)
		logDone(logMessage)
	}
}

// ensure we allow the use of valid tags
func TestTagValidPrefixedRepo(t *testing.T) {
	pullCmd := exec.Command(dockerBinary, "pull", "busybox")
	out, exitCode, err := runCommandWithOutput(pullCmd)
	errorOut(err, t, fmt.Sprintf("%s %s", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("pulling the busybox image from the registry has failed")
	}

	validRepos := []string{"fooo/bar", "fooaa/test"}

	for _, repo := range validRepos {
		tagCmd := exec.Command(dockerBinary, "tag", "busybox", repo)
		_, _, err := runCommandWithOutput(tagCmd)
		if err != nil {
			t.Errorf("tag busybox %v should have worked: %s", repo, err)
			continue
		}
		deleteImages(repo)
		logMessage := fmt.Sprintf("tag - busybox %v", repo)
		logDone(logMessage)
	}
}
