package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestCommitAfterContainerIsDone(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-i", "-a", "stdin", "busybox", "echo", "foo")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("failed to run container: %s, %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	waitCmd := exec.Command(dockerBinary, "wait", cleanedContainerID)
	if _, _, err = runCommandWithOutput(waitCmd); err != nil {
		t.Fatalf("error thrown while waiting for container: %s, %v", out, err)
	}

	commitCmd := exec.Command(dockerBinary, "commit", cleanedContainerID)
	out, _, err = runCommandWithOutput(commitCmd)
	if err != nil {
		t.Fatalf("failed to commit container to image: %s, %v", out, err)
	}

	cleanedImageID := stripTrailingCharacters(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedImageID)
	if out, _, err = runCommandWithOutput(inspectCmd); err != nil {
		t.Fatalf("failed to inspect image: %s, %v", out, err)
	}

	deleteContainer(cleanedContainerID)
	deleteImages(cleanedImageID)

	logDone("commit - echo foo and commit the image")
}

func TestCommitWithoutPause(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-i", "-a", "stdin", "busybox", "echo", "foo")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("failed to run container: %s, %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	waitCmd := exec.Command(dockerBinary, "wait", cleanedContainerID)
	if _, _, err = runCommandWithOutput(waitCmd); err != nil {
		t.Fatalf("error thrown while waiting for container: %s, %v", out, err)
	}

	commitCmd := exec.Command(dockerBinary, "commit", "-p=false", cleanedContainerID)
	out, _, err = runCommandWithOutput(commitCmd)
	if err != nil {
		t.Fatalf("failed to commit container to image: %s, %v", out, err)
	}

	cleanedImageID := stripTrailingCharacters(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedImageID)
	if out, _, err = runCommandWithOutput(inspectCmd); err != nil {
		t.Fatalf("failed to inspect image: %s, %v", out, err)
	}

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
	imageID, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err)
	}
	imageID = strings.Trim(imageID, "\r\n")

	cmd = exec.Command(dockerBinary, "run", imageID, "cat", "/foo")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	if actual := strings.Trim(out, "\r\n"); actual != "koye" {
		t.Fatalf("expected output koye received %q", actual)
	}

	deleteAllContainers()
	deleteImages(imageID)

	logDone("commit - commit file and read")
}

func TestCommitHardlink(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-t", "--name", "hardlinks", "busybox", "sh", "-c", "touch file1 && ln file1 file2 && ls -di file1 file2")
	firstOuput, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err)
	}

	chunks := strings.Split(strings.TrimSpace(firstOuput), " ")
	inode := chunks[0]
	found := false
	for _, chunk := range chunks[1:] {
		if chunk == inode {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Failed to create hardlink in a container. Expected to find %q in %q", inode, chunks[1:])
	}

	cmd = exec.Command(dockerBinary, "commit", "hardlinks", "hardlinks")
	imageID, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(imageID, err)
	}
	imageID = strings.Trim(imageID, "\r\n")

	cmd = exec.Command(dockerBinary, "run", "-t", "hardlinks", "ls", "-di", "file1", "file2")
	secondOuput, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err)
	}

	chunks = strings.Split(strings.TrimSpace(secondOuput), " ")
	inode = chunks[0]
	found = false
	for _, chunk := range chunks[1:] {
		if chunk == inode {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Failed to create hardlink in a container. Expected to find %q in %q", inode, chunks[1:])
	}

	deleteAllContainers()
	deleteImages(imageID)

	logDone("commit - commit hardlinks")
}

func TestCommitTTY(t *testing.T) {
	defer deleteImages("ttytest")
	defer deleteAllContainers()

	cmd := exec.Command(dockerBinary, "run", "-t", "--name", "tty", "busybox", "/bin/ls")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "commit", "tty", "ttytest")
	imageID, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err)
	}
	imageID = strings.Trim(imageID, "\r\n")

	cmd = exec.Command(dockerBinary, "run", "ttytest", "/bin/ls")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	logDone("commit - commit tty")
}

func TestCommitWithHostBindMount(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--name", "bind-commit", "-v", "/dev/null:/winning", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "commit", "bind-commit", "bindtest")
	imageID, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(imageID, err)
	}

	imageID = strings.Trim(imageID, "\r\n")

	cmd = exec.Command(dockerBinary, "run", "bindtest", "true")

	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()
	deleteImages(imageID)

	logDone("commit - commit bind mounted file")
}
