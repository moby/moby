package main

import (
	"bytes"
	"context"
	"os/exec"
	"testing"

	"github.com/moby/moby/v2/integration-cli/cli"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type DockerCLILoginSuite struct {
	ds *DockerSuite
}

func (s *DockerCLILoginSuite) TearDownTest(ctx context.Context, t *testing.T) {
	s.ds.TearDownTest(ctx, t)
}

func (s *DockerCLILoginSuite) OnTimeout(t *testing.T) {
	s.ds.OnTimeout(t)
}

func (s *DockerCLILoginSuite) TestLoginWithoutTTY(c *testing.T) {
	cmd := exec.Command(dockerBinary, "login")

	// Send to stdin so the process does not get the TTY
	cmd.Stdin = bytes.NewBufferString("buffer test string \n")

	// run the command and block until it's done
	err := cmd.Run()
	assert.ErrorContains(c, err, "") // "Expected non nil err when logging in & TTY not available"
}

func (s *DockerRegistryAuthHtpasswdSuite) TestLoginToPrivateRegistry(c *testing.T) {
	// wrong credentials
	out, _, err := dockerCmdWithError("login", "-u", s.reg.Username(), "-p", "WRONGPASSWORD", privateRegistryURL)
	assert.ErrorContains(c, err, "", out)
	assert.Assert(c, is.Contains(out, "401 Unauthorized"))

	// now it's fine
	cli.DockerCmd(c, "login", "-u", s.reg.Username(), "-p", s.reg.Password(), privateRegistryURL)
}
