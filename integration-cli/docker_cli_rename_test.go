package main

import (
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/stringid"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestRenameStoppedContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "--name", "first_name", "-d", "busybox", "sh")

	cleanedContainerID := strings.TrimSpace(out)
	dockerCmd(c, "wait", cleanedContainerID)

	name, err := inspectField(cleanedContainerID, "Name")
	newName := "new_name" + stringid.GenerateNonCryptoID()
	dockerCmd(c, "rename", "first_name", newName)

	name, err = inspectField(cleanedContainerID, "Name")
	c.Assert(err, checker.IsNil, check.Commentf("Failed to rename container %s", name))
	c.Assert(name, checker.Equals, "/"+newName, check.Commentf("Failed to rename container %s", name))

}

func (s *DockerSuite) TestRenameRunningContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "--name", "first_name", "-d", "busybox", "sh")

	newName := "new_name" + stringid.GenerateNonCryptoID()
	cleanedContainerID := strings.TrimSpace(out)
	dockerCmd(c, "rename", "first_name", newName)

	name, err := inspectField(cleanedContainerID, "Name")
	c.Assert(err, checker.IsNil, check.Commentf("Failed to rename container %s", name))
	c.Assert(name, checker.Equals, "/"+newName, check.Commentf("Failed to rename container %s", name))
}

func (s *DockerSuite) TestRenameRunningContainerAndReuse(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "--name", "first_name", "-d", "busybox", "top")
	c.Assert(waitRun("first_name"), check.IsNil)

	newName := "new_name"
	ContainerID := strings.TrimSpace(out)
	dockerCmd(c, "rename", "first_name", newName)

	name, err := inspectField(ContainerID, "Name")
	c.Assert(err, checker.IsNil, check.Commentf("Failed to rename container %s", name))
	c.Assert(name, checker.Equals, "/"+newName, check.Commentf("Failed to rename container"))

	out, _ = dockerCmd(c, "run", "--name", "first_name", "-d", "busybox", "top")
	c.Assert(waitRun("first_name"), check.IsNil)
	newContainerID := strings.TrimSpace(out)
	name, err = inspectField(newContainerID, "Name")
	c.Assert(err, checker.IsNil, check.Commentf("Failed to reuse container name"))
	c.Assert(name, checker.Equals, "/first_name", check.Commentf("Failed to reuse container name"))
}

func (s *DockerSuite) TestRenameCheckNames(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "--name", "first_name", "-d", "busybox", "sh")

	newName := "new_name" + stringid.GenerateNonCryptoID()
	dockerCmd(c, "rename", "first_name", newName)

	name, err := inspectField(newName, "Name")
	c.Assert(err, checker.IsNil, check.Commentf("Failed to rename container %s", name))
	c.Assert(name, checker.Equals, "/"+newName, check.Commentf("Failed to rename container %s", name))

	name, err = inspectField("first_name", "Name")
	c.Assert(err, checker.NotNil, check.Commentf(name))
	c.Assert(err.Error(), checker.Contains, "No such image or container: first_name")
}

func (s *DockerSuite) TestRenameInvalidName(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "--name", "myname", "-d", "busybox", "top")

	out, _, err := dockerCmdWithError("rename", "myname", "new:invalid")
	c.Assert(err, checker.NotNil, check.Commentf("Renaming container to invalid name should have failed: %s", out))
	c.Assert(out, checker.Contains, "Invalid container name", check.Commentf("%v", err))

	out, _, err = dockerCmdWithError("rename", "myname", "")
	c.Assert(err, checker.NotNil, check.Commentf("Renaming container to invalid name should have failed: %s", out))
	c.Assert(out, checker.Contains, "may be empty", check.Commentf("%v", err))

	out, _, err = dockerCmdWithError("rename", "", "newname")
	c.Assert(err, checker.NotNil, check.Commentf("Renaming container with empty name should have failed: %s", out))
	c.Assert(out, checker.Contains, "may be empty", check.Commentf("%v", err))

	out, _ = dockerCmd(c, "ps", "-a")
	c.Assert(out, checker.Contains, "myname", check.Commentf("Output of docker ps should have included 'myname': %s", out))
}
