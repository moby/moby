package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestCommitAfterContainerIsDone(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-i", "-a", "stdin", "busybox", "echo", "foo")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, fmt.Sprintf("failed to run container: %v %v", out, err))

	cleanedContainerID := stripTrailingCharacters(out)

	waitCmd := exec.Command(dockerBinary, "wait", cleanedContainerID)
	_, _, err = runCommandWithOutput(waitCmd)
	errorOut(err, t, fmt.Sprintf("error thrown while waiting for container: %s", out))

	commitCmd := exec.Command(dockerBinary, "commit", cleanedContainerID)
	out, _, err = runCommandWithOutput(commitCmd)
	errorOut(err, t, fmt.Sprintf("failed to commit container to image: %v %v", out, err))

	cleanedImageID := stripTrailingCharacters(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedImageID)
	out, _, err = runCommandWithOutput(inspectCmd)
	errorOut(err, t, fmt.Sprintf("failed to inspect image: %v %v", out, err))

	deleteContainer(cleanedContainerID)
	deleteImages(cleanedImageID)

	logDone("commit - echo foo and commit the image")
}

func TestCommitWithoutPause(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-i", "-a", "stdin", "busybox", "echo", "foo")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, fmt.Sprintf("failed to run container: %v %v", out, err))

	cleanedContainerID := stripTrailingCharacters(out)

	waitCmd := exec.Command(dockerBinary, "wait", cleanedContainerID)
	_, _, err = runCommandWithOutput(waitCmd)
	errorOut(err, t, fmt.Sprintf("error thrown while waiting for container: %s", out))

	commitCmd := exec.Command(dockerBinary, "commit", "-p=false", cleanedContainerID)
	out, _, err = runCommandWithOutput(commitCmd)
	errorOut(err, t, fmt.Sprintf("failed to commit container to image: %v %v", out, err))

	cleanedImageID := stripTrailingCharacters(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedImageID)
	out, _, err = runCommandWithOutput(inspectCmd)
	errorOut(err, t, fmt.Sprintf("failed to inspect image: %v %v", out, err))

	deleteContainer(cleanedContainerID)
	deleteImages(cleanedImageID)

	logDone("commit - echo foo and commit the image with --pause=false")
}

func TestCommitNewFile(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--name", "commiter", "busybox", "/bin/sh", "-c", "echo koye > /foo")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "commit", "commiter")
	imageId, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err)
	}
	imageId = strings.Trim(imageId, "\r\n")

	cmd = exec.Command(dockerBinary, "run", imageId, "cat", "/foo")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	if actual := strings.Trim(out, "\r\n"); actual != "koye" {
		t.Fatalf("expected output koye received %s", actual)
	}

	deleteAllContainers()
	deleteImages(imageId)

	logDone("commit - commit file and read")
}

func TestCommitTTY(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-t", "--name", "tty", "busybox", "/bin/ls")

	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "commit", "tty", "ttytest")
	imageId, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err)
	}
	imageId = strings.Trim(imageId, "\r\n")

	cmd = exec.Command(dockerBinary, "run", "ttytest", "/bin/ls")

	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}
}

func TestCommitWithHostBindMount(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--name", "bind-commit", "-v", "/dev/null:/winning", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "commit", "bind-commit", "bindtest")
	imageId, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err)
	}
	imageId = strings.Trim(imageId, "\r\n")

	cmd = exec.Command(dockerBinary, "run", "bindtest", "true")

	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()
	deleteImages(imageId)

	logDone("commit - commit bind mounted file")
}
