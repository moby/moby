package main

import (
	"bytes"
	"os/exec"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestLoginWithoutTTY(c *check.C) {
	cmd := exec.Command(dockerBinary, "login")

	// Send to stdin so the process does not get the TTY
	cmd.Stdin = bytes.NewBufferString("buffer test string \n")

	// run the command and block until it's done
	err := cmd.Run()
	c.Assert(err, checker.NotNil) //"Expected non nil err when loginning in & TTY not available"
}

func (s *DockerRegistryAuthSuite) TestLoginToPrivateRegistry(c *check.C) {
	// wrong credentials
	out, _, err := dockerCmdWithError("login", "-u", s.reg.username, "-p", "WRONGPASSWORD", privateRegistryURL)
	c.Assert(err, checker.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "401 Unauthorized")

	// now it's fine
	dockerCmd(c, "login", "-u", s.reg.username, "-p", s.reg.password, privateRegistryURL)
}

func (s *DockerRegistryAuthSuite) TestLoginToPrivateRegistryDeprecatedEmailFlag(c *check.C) {
	// Test to make sure login still works with the deprecated -e and --email flags
	// wrong credentials
	out, _, err := dockerCmdWithError("login", "-u", s.reg.username, "-p", "WRONGPASSWORD", "-e", s.reg.email, privateRegistryURL)
	c.Assert(err, checker.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "401 Unauthorized")

	// now it's fine
	// -e flag
	dockerCmd(c, "login", "-u", s.reg.username, "-p", s.reg.password, "-e", s.reg.email, privateRegistryURL)
	// --email flag
	dockerCmd(c, "login", "-u", s.reg.username, "-p", s.reg.password, "--email", s.reg.email, privateRegistryURL)
}
