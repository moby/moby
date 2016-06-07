package main

import (
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/stringid"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestRenameStoppedContainer(c *check.C) {
	out, _ := dockerCmd(c, "run", "--name", "first_name", "-d", "busybox", "sh")

	cleanedContainerID := strings.TrimSpace(out)
	dockerCmd(c, "wait", cleanedContainerID)

	name := inspectField(c, cleanedContainerID, "Name")
	newName := "new_name" + stringid.GenerateNonCryptoID()
	dockerCmd(c, "rename", "first_name", newName)

	name = inspectField(c, cleanedContainerID, "Name")
	c.Assert(name, checker.Equals, "/"+newName, check.Commentf("Failed to rename container %s", name))

}

func (s *DockerSuite) TestRenameRunningContainer(c *check.C) {
	out, _ := dockerCmd(c, "run", "--name", "first_name", "-d", "busybox", "sh")

	newName := "new_name" + stringid.GenerateNonCryptoID()
	cleanedContainerID := strings.TrimSpace(out)
	dockerCmd(c, "rename", "first_name", newName)

	name := inspectField(c, cleanedContainerID, "Name")
	c.Assert(name, checker.Equals, "/"+newName, check.Commentf("Failed to rename container %s", name))
}

func (s *DockerSuite) TestRenameRunningContainerAndReuse(c *check.C) {
	out, _ := runSleepingContainer(c, "--name", "first_name")
	c.Assert(waitRun("first_name"), check.IsNil)

	newName := "new_name"
	ContainerID := strings.TrimSpace(out)
	dockerCmd(c, "rename", "first_name", newName)

	name := inspectField(c, ContainerID, "Name")
	c.Assert(name, checker.Equals, "/"+newName, check.Commentf("Failed to rename container"))

	out, _ = runSleepingContainer(c, "--name", "first_name")
	c.Assert(waitRun("first_name"), check.IsNil)
	newContainerID := strings.TrimSpace(out)
	name = inspectField(c, newContainerID, "Name")
	c.Assert(name, checker.Equals, "/first_name", check.Commentf("Failed to reuse container name"))
}

func (s *DockerSuite) TestRenameCheckNames(c *check.C) {
	dockerCmd(c, "run", "--name", "first_name", "-d", "busybox", "sh")

	newName := "new_name" + stringid.GenerateNonCryptoID()
	dockerCmd(c, "rename", "first_name", newName)

	name := inspectField(c, newName, "Name")
	c.Assert(name, checker.Equals, "/"+newName, check.Commentf("Failed to rename container %s", name))

	name, err := inspectFieldWithError("first_name", "Name")
	c.Assert(err, checker.NotNil, check.Commentf(name))
	c.Assert(err.Error(), checker.Contains, "No such image or container: first_name")
}

func (s *DockerSuite) TestRenameInvalidName(c *check.C) {
	runSleepingContainer(c, "--name", "myname")

	out, _, err := dockerCmdWithError("rename", "myname", "new:invalid")
	c.Assert(err, checker.NotNil, check.Commentf("Renaming container to invalid name should have failed: %s", out))
	c.Assert(out, checker.Contains, "Invalid container name", check.Commentf("%v", err))

	out, _, err = dockerCmdWithError("rename", "myname")
	c.Assert(err, checker.NotNil, check.Commentf("Renaming container to invalid name should have failed: %s", out))
	c.Assert(out, checker.Contains, "requires exactly 2 argument(s).", check.Commentf("%v", err))

	out, _, err = dockerCmdWithError("rename", "myname", "")
	c.Assert(err, checker.NotNil, check.Commentf("Renaming container to invalid name should have failed: %s", out))
	c.Assert(out, checker.Contains, "may be empty", check.Commentf("%v", err))

	out, _, err = dockerCmdWithError("rename", "", "newname")
	c.Assert(err, checker.NotNil, check.Commentf("Renaming container with empty name should have failed: %s", out))
	c.Assert(out, checker.Contains, "may be empty", check.Commentf("%v", err))

	out, _ = dockerCmd(c, "ps", "-a")
	c.Assert(out, checker.Contains, "myname", check.Commentf("Output of docker ps should have included 'myname': %s", out))
}

func (s *DockerSuite) TestRenameAnonymousContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)

	dockerCmd(c, "network", "create", "network1")
	out, _ := dockerCmd(c, "create", "-it", "--net", "network1", "busybox", "top")

	anonymousContainerID := strings.TrimSpace(out)

	dockerCmd(c, "rename", anonymousContainerID, "container1")
	dockerCmd(c, "start", "container1")

	count := "-c"
	if daemonPlatform == "windows" {
		count = "-n"
	}

	_, _, err := dockerCmdWithError("run", "--net", "network1", "busybox", "ping", count, "1", "container1")
	c.Assert(err, check.IsNil, check.Commentf("Embedded DNS lookup fails after renaming anonymous container: %v", err))
}
