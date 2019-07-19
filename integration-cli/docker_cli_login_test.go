package main

import (
	"bytes"
	"os/exec"
	"strings"

	"github.com/go-check/check"
	"gotest.tools/assert"
)

func (s *DockerSuite) TestLoginWithoutTTY(c *check.C) {
	cmd := exec.Command(dockerBinary, "login")

	// Send to stdin so the process does not get the TTY
	cmd.Stdin = bytes.NewBufferString("buffer test string \n")

	// run the command and block until it's done
	err := cmd.Run()
	assert.ErrorContains(c, err, "") //"Expected non nil err when logging in & TTY not available"
}

func (s *DockerRegistryAuthHtpasswdSuite) TestLoginToPrivateRegistry(c *check.C) {
	// wrong credentials
	s.d.Start(c)
	out, err := s.d.Cmd("login", "-u", s.reg.Username(), "-p", "WRONGPASSWORD", s.reg.URL())
	assert.ErrorContains(c, err, "", out)
	assert.Assert(c, strings.Contains(out, "401 Unauthorized"), out)

	// now it's fine
	s.d.CmdT(c, "login", "-u", s.reg.Username(), "-p", s.reg.Password(), s.reg.URL())
}
