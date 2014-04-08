package main

import (
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/runtime"
	"os/exec"
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

func TestCommitNoMerge(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "-e", "foo=bar", "busybox", "/bin/true")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, fmt.Sprintf("failed to run container: %v %v", out, err))

	cleanedContainerID := stripTrailingCharacters(out)

	waitCmd := exec.Command(dockerBinary, "wait", cleanedContainerID)
	_, _, err = runCommandWithOutput(waitCmd)
	errorOut(err, t, fmt.Sprintf("error thrown while waiting for container: %s", out))

	commitCmd := exec.Command(dockerBinary, "commit", "--no-merge", cleanedContainerID)
	out, _, err = runCommandWithOutput(commitCmd)
	errorOut(err, t, fmt.Sprintf("failed to commit container to image: %v %v", out, err))

	cleanedImageID := stripTrailingCharacters(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedImageID)
	out, _, err = runCommandWithOutput(inspectCmd)
	errorOut(err, t, fmt.Sprintf("failed to inspect image: %v %v", out, err))

	container := []*runtime.Container{
		&runtime.Container{},
	}
	if err := json.Unmarshal([]byte(out), &container); err != nil {
		t.Fatal(err)
	}
	if container[0].Config != nil && container[0].Config.Env != nil {
		found := false
		for _, val := range container[0].Config.Env {
			if val == "foo=bar" {
				found = true
			}
		}
		if found {
			t.Fatal("The parent config propagated to the child image.")
		}
	}

	deleteContainer(cleanedContainerID)
	deleteImages(cleanedImageID)

	logDone("commit - set env, commit with --no-merge and check the image")
}

func TestCommitMerge(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "-e", "foo=bar", "busybox", "/bin/true")
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

	container := []*runtime.Container{
		&runtime.Container{},
	}
	if err := json.Unmarshal([]byte(out), &container); err != nil {
		t.Fatal(err)
	}
	if container[0].Config == nil || container[0].Config.Env == nil || len(container[0].Config.Env) == 0 {
		t.Fatal("The parent config did not propagate to the child image.", container[0].Config.Env, len(container[0].Config.Env))
	}
	if expected := "foo=bar"; container[0].Config.Env[0] != expected {
		t.Fatal("The config merge failed. Expected: %s, Received: %s", expected, container[0].Config.Env[0], out)
	}

	deleteContainer(cleanedContainerID)
	deleteImages(cleanedImageID)

	logDone("commit - set env, commit without --no-merge and check the image")
}
