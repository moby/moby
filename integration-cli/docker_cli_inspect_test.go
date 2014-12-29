package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestInspectImage(t *testing.T) {
	imageTest := "emptyfs"
	imageTestID := "511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158"
	imagesCmd := exec.Command(dockerBinary, "inspect", "--format='{{.Id}}'", imageTest)
	out, exitCode, err := runCommandWithOutput(imagesCmd)
	if exitCode != 0 || err != nil {
		t.Fatalf("failed to inspect image: %s, %v", out, err)
	}

	if id := strings.TrimSuffix(out, "\n"); id != imageTestID {
		t.Fatalf("Expected id: %s for image: %s but received id: %s", imageTestID, imageTest, id)
	}

	logDone("inspect - inspect an image")
}

func TestInspectExecID(t *testing.T) {
	defer deleteAllContainers()

	out, exitCode, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "busybox", "top"))
	if exitCode != 0 || err != nil {
		t.Fatalf("failed to run container: %s, %v", out, err)
	}
	id := strings.TrimSuffix(out, "\n")

	out, err = inspectField(id, "ExecIDs")
	if err != nil {
		t.Fatalf("failed to inspect container: %s, %v", out, err)
	}
	if out != "<no value>" {
		t.Fatalf("ExecIDs should be empty, got: %s", out)
	}

	exitCode, err = runCommand(exec.Command(dockerBinary, "exec", "-d", id, "ls", "/"))
	if exitCode != 0 || err != nil {
		t.Fatalf("failed to exec in container: %s, %v", out, err)
	}

	out, err = inspectField(id, "ExecIDs")
	if err != nil {
		t.Fatalf("failed to inspect container: %s, %v", out, err)
	}

	out = strings.TrimSuffix(out, "\n")
	if out == "[]" || out == "<no value>" {
		t.Fatalf("ExecIDs should not be empty, got: %s", out)
	}

	logDone("inspect - inspect a container with ExecIDs")
}
