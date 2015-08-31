package main

import (
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestVolumeCliCreate(c *check.C) {
	dockerCmd(c, "volume", "create")

	_, err := runCommand(exec.Command(dockerBinary, "volume", "create", "-d", "nosuchdriver"))
	c.Assert(err, check.Not(check.IsNil))

	out, _ := dockerCmd(c, "volume", "create", "--name=test")
	name := strings.TrimSpace(out)
	c.Assert(name, check.Equals, "test")
}

func (s *DockerSuite) TestVolumeCliInspect(c *check.C) {
	c.Assert(
		exec.Command(dockerBinary, "volume", "inspect", "doesntexist").Run(),
		check.Not(check.IsNil),
		check.Commentf("volume inspect should error on non-existant volume"),
	)

	out, _ := dockerCmd(c, "volume", "create")
	name := strings.TrimSpace(out)
	out, _ = dockerCmd(c, "volume", "inspect", "--format='{{ .Name }}'", name)
	c.Assert(strings.TrimSpace(out), check.Equals, name)

	dockerCmd(c, "volume", "create", "--name", "test")
	out, _ = dockerCmd(c, "volume", "inspect", "--format='{{ .Name }}'", "test")
	c.Assert(strings.TrimSpace(out), check.Equals, "test")
}

func (s *DockerSuite) TestVolumeCliLs(c *check.C) {
	out, _ := dockerCmd(c, "volume", "create")
	id := strings.TrimSpace(out)

	dockerCmd(c, "volume", "create", "--name", "test")
	dockerCmd(c, "run", "-v", "/foo", "busybox", "ls", "/")

	out, _ = dockerCmd(c, "volume", "ls")
	outArr := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(outArr), check.Equals, 4, check.Commentf("\n%s", out))

	// Since there is no guarentee of ordering of volumes, we just make sure the names are in the output
	c.Assert(strings.Contains(out, id+"\n"), check.Equals, true)
	c.Assert(strings.Contains(out, "test\n"), check.Equals, true)
}

func (s *DockerSuite) TestVolumeCliRm(c *check.C) {
	out, _ := dockerCmd(c, "volume", "create")
	id := strings.TrimSpace(out)

	dockerCmd(c, "volume", "create", "--name", "test")
	dockerCmd(c, "volume", "rm", id)
	dockerCmd(c, "volume", "rm", "test")

	out, _ = dockerCmd(c, "volume", "ls")
	outArr := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(outArr), check.Equals, 1, check.Commentf("%s\n", out))

	volumeID := "testing"
	dockerCmd(c, "run", "-v", volumeID+":/foo", "--name=test", "busybox", "sh", "-c", "echo hello > /foo/bar")
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "volume", "rm", "testing"))
	c.Assert(
		err,
		check.Not(check.IsNil),
		check.Commentf("Should not be able to remove volume that is in use by a container\n%s", out))

	out, _ = dockerCmd(c, "run", "--volumes-from=test", "--name=test2", "busybox", "sh", "-c", "cat /foo/bar")
	c.Assert(strings.TrimSpace(out), check.Equals, "hello")
	dockerCmd(c, "rm", "-fv", "test2")
	dockerCmd(c, "volume", "inspect", volumeID)
	dockerCmd(c, "rm", "-f", "test")

	out, _ = dockerCmd(c, "run", "--name=test2", "-v", volumeID+":/foo", "busybox", "sh", "-c", "cat /foo/bar")
	c.Assert(strings.TrimSpace(out), check.Equals, "hello", check.Commentf("volume data was removed"))
	dockerCmd(c, "rm", "test2")

	dockerCmd(c, "volume", "rm", volumeID)
	c.Assert(
		exec.Command("volume", "rm", "doesntexist").Run(),
		check.Not(check.IsNil),
		check.Commentf("volume rm should fail with non-existant volume"),
	)
}

func (s *DockerSuite) TestVolumeCliNoArgs(c *check.C) {
	out, _ := dockerCmd(c, "volume")
	// no args should produce the `volume ls` output
	c.Assert(strings.Contains(out, "DRIVER"), check.Equals, true)

	// invalid arg should error and show the command usage on stderr
	_, stderr, _, err := runCommandWithStdoutStderr(exec.Command(dockerBinary, "volume", "somearg"))
	c.Assert(err, check.NotNil)

	expected := "Usage:	docker volume [OPTIONS] [COMMAND]"
	c.Assert(strings.Contains(stderr, expected), check.Equals, true)
}
