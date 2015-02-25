package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestRenameStoppedContainer(t *testing.T) {
	defer deleteAllContainers()

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
	if name != "/new_name" {
		t.Fatal("Failed to rename container ", name)
	}

	logDone("rename - stopped container")
}

func TestRenameRunningContainer(t *testing.T) {
	defer deleteAllContainers()

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
	if name != "/new_name" {
		t.Fatal("Failed to rename container ")
	}

	logDone("rename - running container")
}

func TestRenameCheckNames(t *testing.T) {
	defer deleteAllContainers()

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
	if name != "/new_name" {
		t.Fatal("Failed to rename container ")
	}

	name, err = inspectField("first_name", "Name")
	if err == nil && !strings.Contains(err.Error(), "No such image or container: first_name") {
		t.Fatal(err)
	}

	logDone("rename - running container")
}

func TestRenameInvalidName(t *testing.T) {
	defer deleteAllContainers()
	runCmd := exec.Command(dockerBinary, "run", "--name", "myname", "-d", "busybox", "top")
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		t.Fatalf(out, err)
	}

	runCmd = exec.Command(dockerBinary, "rename", "myname", "new:invalid")
	if out, _, err := runCommandWithOutput(runCmd); err == nil || !strings.Contains(out, "Invalid container name") {
		t.Fatalf("Renaming container to invalid name should have failed: %s\n%v", out, err)
	}

	runCmd = exec.Command(dockerBinary, "ps", "-a")
	if out, _, err := runCommandWithOutput(runCmd); err != nil || !strings.Contains(out, "myname") {
		t.Fatalf("Output of docker ps should have included 'myname': %s\n%v", out, err)
	}

	logDone("rename - invalid container name")
}
