package main

import (
	"bytes"
	"os/exec"

	"github.com/docker/docker/integration-cli/checker"
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

func (s *DockerRegistryAuthHtpasswdSuite) TestLoginToPrivateRegistry(c *check.C) {
	// wrong credentials
	out, _, err := dockerCmdWithError("login", "-u", s.reg.Username(), "-p", "WRONGPASSWORD", privateRegistryURL)
	c.Assert(err, checker.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "401 Unauthorized")

	// now it's fine
	dockerCmd(c, "login", "-u", s.reg.Username(), "-p", s.reg.Password(), privateRegistryURL)
}

func (s *DockerRegistryAuthHtpasswdSuite) TestLoginToPrivateRegistryDeprecatedEmailFlag(c *check.C) {
	// Test to make sure login still works with the deprecated -e and --email flags
	// wrong credentials
	out, _, err := dockerCmdWithError("login", "-u", s.reg.Username(), "-p", "WRONGPASSWORD", "-e", s.reg.Email(), privateRegistryURL)
	c.Assert(err, checker.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "401 Unauthorized")

	// now it's fine
	// -e flag
	dockerCmd(c, "login", "-u", s.reg.Username(), "-p", s.reg.Password(), "-e", s.reg.Email(), privateRegistryURL)
	// --email flag
	dockerCmd(c, "login", "-u", s.reg.Username(), "-p", s.reg.Password(), "--email", s.reg.Email(), privateRegistryURL)
}
