package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// tagging a named image in a new unprefixed repo should work
func TestTagUnprefixedRepoByName(t *testing.T) {
	if err := pullImageIfNotExist("busybox:latest"); err != nil {
		t.Fatal("couldn't find the busybox:latest image locally and failed to pull it")
	}

	tagCmd := exec.Command(dockerBinary, "tag", "busybox:latest", "testfoobarbaz")
	if out, _, err := runCommandWithOutput(tagCmd); err != nil {
		t.Fatal(out, err)
	}

	deleteImages("testfoobarbaz")

	logDone("tag - busybox -> testfoobarbaz")
}

// tagging an image by ID in a new unprefixed repo should work
func TestTagUnprefixedRepoByID(t *testing.T) {
	getIDCmd := exec.Command(dockerBinary, "inspect", "-f", "{{.Id}}", "busybox")
	out, _, err := runCommandWithOutput(getIDCmd)
	if err != nil {
		t.Fatalf("failed to get the image ID of busybox: %s, %v", out, err)
	}

	cleanedImageID := stripTrailingCharacters(out)
	tagCmd := exec.Command(dockerBinary, "tag", cleanedImageID, "testfoobarbaz")
	if out, _, err = runCommandWithOutput(tagCmd); err != nil {
		t.Fatal(out, err)
	}

	deleteImages("testfoobarbaz")

	logDone("tag - busybox's image ID -> testfoobarbaz")
}

// ensure we don't allow the use of invalid repository names; these tag operations should fail
func TestTagInvalidUnprefixedRepo(t *testing.T) {

	invalidRepos := []string{"fo$z$", "Foo@3cc", "Foo$3", "Foo*3", "Fo^3", "Foo!3", "F)xcz(", "fo%asd"}

	for _, repo := range invalidRepos {
		tagCmd := exec.Command(dockerBinary, "tag", "busybox", repo)
		_, _, err := runCommandWithOutput(tagCmd)
		if err == nil {
			t.Fatalf("tag busybox %v should have failed", repo)
		}
	}
	logDone("tag - busybox invalid repo names --> must fail")
}

// ensure we don't allow the use of invalid tags; these tag operations should fail
func TestTagInvalidPrefixedRepo(t *testing.T) {
	long_tag := makeRandomString(121)

	invalidTags := []string{"repo:fo$z$", "repo:Foo@3cc", "repo:Foo$3", "repo:Foo*3", "repo:Fo^3", "repo:Foo!3", "repo:%goodbye", "repo:#hashtagit", "repo:F)xcz(", "repo:-foo", "repo:..", long_tag}

	for _, repotag := range invalidTags {
		tagCmd := exec.Command(dockerBinary, "tag", "busybox", repotag)
		_, _, err := runCommandWithOutput(tagCmd)
		if err == nil {
			t.Fatalf("tag busybox %v should have failed", repotag)
		}
	}
	logDone("tag - busybox with invalid repo:tagnames --> must fail")
}

// ensure we allow the use of valid tags
func TestTagValidPrefixedRepo(t *testing.T) {
	if err := pullImageIfNotExist("busybox:latest"); err != nil {
		t.Fatal("couldn't find the busybox:latest image locally and failed to pull it")
	}

	validRepos := []string{"fooo/bar", "fooaa/test", "foooo:t"}

	for _, repo := range validRepos {
		tagCmd := exec.Command(dockerBinary, "tag", "busybox:latest", repo)
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

// tag an image with an existed tag name without -f option should fail
func TestTagExistedNameWithoutForce(t *testing.T) {
	if err := pullImageIfNotExist("busybox:latest"); err != nil {
		t.Fatal("couldn't find the busybox:latest image locally and failed to pull it")
	}

	tagCmd := exec.Command(dockerBinary, "tag", "busybox:latest", "busybox:test")
	if out, _, err := runCommandWithOutput(tagCmd); err != nil {
		t.Fatal(out, err)
	}
	tagCmd = exec.Command(dockerBinary, "tag", "busybox:latest", "busybox:test")
	out, _, err := runCommandWithOutput(tagCmd)
	if err == nil || !strings.Contains(out, "Conflict: Tag test is already set to image") {
		t.Fatal("tag busybox busybox:test should have failed,because busybox:test is existed")
	}
	deleteImages("busybox:test")

	logDone("tag - busybox with an existed tag name without -f option --> must fail")
}

// tag an image with an existed tag name with -f option should work
func TestTagExistedNameWithForce(t *testing.T) {
	if err := pullImageIfNotExist("busybox:latest"); err != nil {
		t.Fatal("couldn't find the busybox:latest image locally and failed to pull it")
	}

	tagCmd := exec.Command(dockerBinary, "tag", "busybox:latest", "busybox:test")
	if out, _, err := runCommandWithOutput(tagCmd); err != nil {
		t.Fatal(out, err)
	}
	tagCmd = exec.Command(dockerBinary, "tag", "-f", "busybox:latest", "busybox:test")
	if out, _, err := runCommandWithOutput(tagCmd); err != nil {
		t.Fatal(out, err)
	}
	deleteImages("busybox:test")

	logDone("tag - busybox with an existed tag name with -f option work")
}
