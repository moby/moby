package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestRenameStoppedContainer(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "--name", "first_name", "-d", "busybox", "sh")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf(out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	runCmd = exec.Command(dockerBinary, "wait", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf(out, err)
	}

	name, err := inspectField(cleanedContainerID, "Name")

	runCmd = exec.Command(dockerBinary, "rename", "first_name", "new_name")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf(out, err)
	}

	name, err = inspectField(cleanedContainerID, "Name")
	if err != nil {
		t.Fatal(err)
	}
	if name != "new_name" {
		t.Fatal("Failed to rename container ", name)
	}
	deleteAllContainers()

	logDone("rename - stopped container")
}

func TestRenameRunningContainer(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "--name", "first_name", "-d", "busybox", "sh")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf(out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)
	runCmd = exec.Command(dockerBinary, "rename", "first_name", "new_name")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf(out, err)
	}

	name, err := inspectField(cleanedContainerID, "Name")
	if err != nil {
		t.Fatal(err)
	}
	if name != "new_name" {
		t.Fatal("Failed to rename container ")
	}
	deleteAllContainers()

	logDone("rename - running container")
}

func TestRenameCheckNames(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "--name", "first_name", "-d", "busybox", "sh")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf(out, err)
	}

	runCmd = exec.Command(dockerBinary, "rename", "first_name", "new_name")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf(out, err)
	}

	name, err := inspectField("new_name", "Name")
	if err != nil {
		t.Fatal(err)
	}
	if name != "new_name" {
		t.Fatal("Failed to rename container ")
	}

	name, err = inspectField("first_name", "Name")
	if err == nil && !strings.Contains(err.Error(), "No such image or container: first_name") {
		t.Fatal(err)
	}

	deleteAllContainers()

	logDone("rename - running container")
}
