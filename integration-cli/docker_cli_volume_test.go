package main

import (
	"os/exec"
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
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

func (s *DockerSuite) TestVolumeCliCreateOptionConflict(c *check.C) {
	dockerCmd(c, "volume", "create", "--name=test")
	out, _, err := dockerCmdWithError("volume", "create", "--name", "test", "--driver", "nosuchdriver")
	c.Assert(err, check.NotNil, check.Commentf("volume create exception name already in use with another driver"))
	c.Assert(out, checker.Contains, "A volume named test already exists")

	out, _ = dockerCmd(c, "volume", "inspect", "--format='{{ .Driver }}'", "test")
	_, _, err = dockerCmdWithError("volume", "create", "--name", "test", "--driver", strings.TrimSpace(out))
	c.Assert(err, check.IsNil)
}

func (s *DockerSuite) TestVolumeCliInspect(c *check.C) {
	c.Assert(
		exec.Command(dockerBinary, "volume", "inspect", "doesntexist").Run(),
		check.Not(check.IsNil),
		check.Commentf("volume inspect should error on non-existent volume"),
	)

	out, _ := dockerCmd(c, "volume", "create")
	name := strings.TrimSpace(out)
	out, _ = dockerCmd(c, "volume", "inspect", "--format='{{ .Name }}'", name)
	c.Assert(strings.TrimSpace(out), check.Equals, name)

	dockerCmd(c, "volume", "create", "--name", "test")
	out, _ = dockerCmd(c, "volume", "inspect", "--format='{{ .Name }}'", "test")
	c.Assert(strings.TrimSpace(out), check.Equals, "test")
}

func (s *DockerSuite) TestVolumeCliInspectMulti(c *check.C) {
	dockerCmd(c, "volume", "create", "--name", "test1")
	dockerCmd(c, "volume", "create", "--name", "test2")
	dockerCmd(c, "volume", "create", "--name", "not-shown")

	out, _, err := dockerCmdWithError("volume", "inspect", "--format='{{ .Name }}'", "test1", "test2", "doesntexist", "not-shown")
	c.Assert(err, checker.NotNil)
	outArr := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(outArr), check.Equals, 3, check.Commentf("\n%s", out))

	c.Assert(out, checker.Contains, "test1")
	c.Assert(out, checker.Contains, "test2")
	c.Assert(out, checker.Contains, "Error: No such volume: doesntexist")
	c.Assert(out, checker.Not(checker.Contains), "not-shown")
}

func (s *DockerSuite) TestVolumeCliLs(c *check.C) {
	prefix := ""
	if daemonPlatform == "windows" {
		prefix = "c:"
	}
	out, _ := dockerCmd(c, "volume", "create")
	id := strings.TrimSpace(out)

	dockerCmd(c, "volume", "create", "--name", "test")
	dockerCmd(c, "run", "-v", prefix+"/foo", "busybox", "ls", "/")

	out, _ = dockerCmd(c, "volume", "ls")
	outArr := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(outArr), check.Equals, 4, check.Commentf("\n%s", out))

	// Since there is no guarantee of ordering of volumes, we just make sure the names are in the output
	c.Assert(strings.Contains(out, id+"\n"), check.Equals, true)
	c.Assert(strings.Contains(out, "test\n"), check.Equals, true)
}

func (s *DockerSuite) TestVolumeCliLsFilterDangling(c *check.C) {
	prefix := ""
	if daemonPlatform == "windows" {
		prefix = "c:"
	}
	dockerCmd(c, "volume", "create", "--name", "testnotinuse1")
	dockerCmd(c, "volume", "create", "--name", "testisinuse1")
	dockerCmd(c, "volume", "create", "--name", "testisinuse2")

	// Make sure both "created" (but not started), and started
	// containers are included in reference counting
	dockerCmd(c, "run", "--name", "volume-test1", "-v", "testisinuse1:"+prefix+"/foo", "busybox", "true")
	dockerCmd(c, "create", "--name", "volume-test2", "-v", "testisinuse2:"+prefix+"/foo", "busybox", "true")

	out, _ := dockerCmd(c, "volume", "ls")

	// No filter, all volumes should show
	c.Assert(out, checker.Contains, "testnotinuse1\n", check.Commentf("expected volume 'testnotinuse1' in output"))
	c.Assert(out, checker.Contains, "testisinuse1\n", check.Commentf("expected volume 'testisinuse1' in output"))
	c.Assert(out, checker.Contains, "testisinuse2\n", check.Commentf("expected volume 'testisinuse2' in output"))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "dangling=false")

	// Explicitly disabling dangling
	c.Assert(out, check.Not(checker.Contains), "testnotinuse1\n", check.Commentf("expected volume 'testnotinuse1' in output"))
	c.Assert(out, checker.Contains, "testisinuse1\n", check.Commentf("expected volume 'testisinuse1' in output"))
	c.Assert(out, checker.Contains, "testisinuse2\n", check.Commentf("expected volume 'testisinuse2' in output"))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "dangling=true")

	// Filter "dangling" volumes; only "dangling" (unused) volumes should be in the output
	c.Assert(out, checker.Contains, "testnotinuse1\n", check.Commentf("expected volume 'testnotinuse1' in output"))
	c.Assert(out, check.Not(checker.Contains), "testisinuse1\n", check.Commentf("volume 'testisinuse1' in output, but not expected"))
	c.Assert(out, check.Not(checker.Contains), "testisinuse2\n", check.Commentf("volume 'testisinuse2' in output, but not expected"))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "dangling=1")
	// Filter "dangling" volumes; only "dangling" (unused) volumes should be in the output, dangling also accept 1
	c.Assert(out, checker.Contains, "testnotinuse1\n", check.Commentf("expected volume 'testnotinuse1' in output"))
	c.Assert(out, check.Not(checker.Contains), "testisinuse1\n", check.Commentf("volume 'testisinuse1' in output, but not expected"))
	c.Assert(out, check.Not(checker.Contains), "testisinuse2\n", check.Commentf("volume 'testisinuse2' in output, but not expected"))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "dangling=0")
	// dangling=0 is same as dangling=false case
	c.Assert(out, check.Not(checker.Contains), "testnotinuse1\n", check.Commentf("expected volume 'testnotinuse1' in output"))
	c.Assert(out, checker.Contains, "testisinuse1\n", check.Commentf("expected volume 'testisinuse1' in output"))
	c.Assert(out, checker.Contains, "testisinuse2\n", check.Commentf("expected volume 'testisinuse2' in output"))
}

func (s *DockerSuite) TestVolumeCliLsErrorWithInvalidFilterName(c *check.C) {
	out, _, err := dockerCmdWithError("volume", "ls", "-f", "FOO=123")
	c.Assert(err, checker.NotNil)
	c.Assert(out, checker.Contains, "Invalid filter")
}

func (s *DockerSuite) TestVolumeCliLsWithIncorrectFilterValue(c *check.C) {
	out, _, err := dockerCmdWithError("volume", "ls", "-f", "dangling=invalid")
	c.Assert(err, check.NotNil)
	c.Assert(out, checker.Contains, "Invalid filter")
}

func (s *DockerSuite) TestVolumeCliRm(c *check.C) {
	prefix := ""
	if daemonPlatform == "windows" {
		prefix = "c:"
	}
	out, _ := dockerCmd(c, "volume", "create")
	id := strings.TrimSpace(out)

	dockerCmd(c, "volume", "create", "--name", "test")
	dockerCmd(c, "volume", "rm", id)
	dockerCmd(c, "volume", "rm", "test")

	out, _ = dockerCmd(c, "volume", "ls")
	outArr := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(outArr), check.Equals, 1, check.Commentf("%s\n", out))

	volumeID := "testing"
	dockerCmd(c, "run", "-v", volumeID+":"+prefix+"/foo", "--name=test", "busybox", "sh", "-c", "echo hello > /foo/bar")
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

	out, _ = dockerCmd(c, "run", "--name=test2", "-v", volumeID+":"+prefix+"/foo", "busybox", "sh", "-c", "cat /foo/bar")
	c.Assert(strings.TrimSpace(out), check.Equals, "hello", check.Commentf("volume data was removed"))
	dockerCmd(c, "rm", "test2")

	dockerCmd(c, "volume", "rm", volumeID)
	c.Assert(
		exec.Command("volume", "rm", "doesntexist").Run(),
		check.Not(check.IsNil),
		check.Commentf("volume rm should fail with non-existent volume"),
	)
}

func (s *DockerSuite) TestVolumeCliNoArgs(c *check.C) {
	out, _ := dockerCmd(c, "volume")
	// no args should produce the cmd usage output
	usage := "Usage:	docker volume [OPTIONS] [COMMAND]"
	c.Assert(out, checker.Contains, usage)

	// invalid arg should error and show the command usage on stderr
	_, stderr, _, err := runCommandWithStdoutStderr(exec.Command(dockerBinary, "volume", "somearg"))
	c.Assert(err, check.NotNil, check.Commentf(stderr))
	c.Assert(stderr, checker.Contains, usage)

	// invalid flag should error and show the flag error and cmd usage
	_, stderr, _, err = runCommandWithStdoutStderr(exec.Command(dockerBinary, "volume", "--no-such-flag"))
	c.Assert(err, check.NotNil, check.Commentf(stderr))
	c.Assert(stderr, checker.Contains, usage)
	c.Assert(stderr, checker.Contains, "flag provided but not defined: --no-such-flag")
}

func (s *DockerSuite) TestVolumeCliInspectTmplError(c *check.C) {
	out, _ := dockerCmd(c, "volume", "create")
	name := strings.TrimSpace(out)

	out, exitCode, err := dockerCmdWithError("volume", "inspect", "--format='{{ .FooBar }}'", name)
	c.Assert(err, checker.NotNil, check.Commentf("Output: %s", out))
	c.Assert(exitCode, checker.Equals, 1, check.Commentf("Output: %s", out))
	c.Assert(out, checker.Contains, "Template parsing error")
}
