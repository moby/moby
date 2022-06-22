package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

type DockerCLILoginSuite struct {
	ds *DockerSuite
}

func (s *DockerCLILoginSuite) TearDownTest(c *testing.T) {
	s.ds.TearDownTest(c)
}

func (s *DockerCLILoginSuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

func (s *DockerCLILoginSuite) TestLoginWithoutTTY(c *testing.T) {
	cmd := exec.Command(dockerBinary, "login")

	// Send to stdin so the process does not get the TTY
	cmd.Stdin = bytes.NewBufferString("buffer test string \n")

	// run the command and block until it's done
	err := cmd.Run()
	assert.ErrorContains(c, err, "") //"Expected non nil err when logging in & TTY not available"
}

func (s *DockerRegistryAuthHtpasswdSuite) TestLoginToPrivateRegistry(c *testing.T) {
	// wrong credentials
	out, _, err := dockerCmdWithError("login", "-u", s.reg.Username(), "-p", "WRONGPASSWORD", privateRegistryURL)
	assert.ErrorContains(c, err, "", out)
	assert.Assert(c, strings.Contains(out, "401 Unauthorized"))

	// now it's fine
	dockerCmd(c, "login", "-u", s.reg.Username(), "-p", s.reg.Password(), privateRegistryURL)
}
