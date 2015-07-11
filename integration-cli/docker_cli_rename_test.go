package main

import (
	"os/exec"
	"strings"

	"github.com/docker/docker/pkg/stringid"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestRenameStoppedContainer(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "--name", "first_name", "-d", "busybox", "sh")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf(out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)

	runCmd = exec.Command(dockerBinary, "wait", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf(out, err)
	}

	name, err := inspectField(cleanedContainerID, "Name")

	newName := "new_name" + stringid.GenerateRandomID()
	runCmd = exec.Command(dockerBinary, "rename", "first_name", newName)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf(out, err)
	}

	name, err = inspectField(cleanedContainerID, "Name")
	if err != nil {
		c.Fatal(err)
	}
	if name != "/"+newName {
		c.Fatal("Failed to rename container ", name)
	}

}

func (s *DockerSuite) TestRenameRunningContainer(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "--name", "first_name", "-d", "busybox", "sh")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf(out, err)
	}

	newName := "new_name" + stringid.GenerateRandomID()
	cleanedContainerID := strings.TrimSpace(out)
	runCmd = exec.Command(dockerBinary, "rename", "first_name", newName)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf(out, err)
	}

	name, err := inspectField(cleanedContainerID, "Name")
	if err != nil {
		c.Fatal(err)
	}
	if name != "/"+newName {
		c.Fatal("Failed to rename container ")
	}
}

func (s *DockerSuite) TestRenameCheckNames(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "--name", "first_name", "-d", "busybox", "sh")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf(out, err)
	}

	newName := "new_name" + stringid.GenerateRandomID()
	runCmd = exec.Command(dockerBinary, "rename", "first_name", newName)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf(out, err)
	}

	name, err := inspectField(newName, "Name")
	if err != nil {
		c.Fatal(err)
	}
	if name != "/"+newName {
		c.Fatal("Failed to rename container ")
	}

	name, err = inspectField("first_name", "Name")
	if err == nil && !strings.Contains(err.Error(), "No such image or container: first_name") {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestRenameInvalidName(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "--name", "myname", "-d", "busybox", "top")
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		c.Fatalf(out, err)
	}

	runCmd = exec.Command(dockerBinary, "rename", "myname", "new:invalid")
	if out, _, err := runCommandWithOutput(runCmd); err == nil || !strings.Contains(out, "Invalid container name") {
		c.Fatalf("Renaming container to invalid name should have failed: %s\n%v", out, err)
	}

	runCmd = exec.Command(dockerBinary, "ps", "-a")
	if out, _, err := runCommandWithOutput(runCmd); err != nil || !strings.Contains(out, "myname") {
		c.Fatalf("Output of docker ps should have included 'myname': %s\n%v", out, err)
	}
}
