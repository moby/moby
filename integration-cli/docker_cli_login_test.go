package main

import (
	"bytes"
	"os/exec"

	"github.com/docker/docker/integration-cli/checker"
	"github.com/go-check/check"
	"io/ioutil"
	"os"
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

func (s *DockerRegistryAuthHtpasswdSuite) TestLoginToPrivateRegistryWithPasswordFile(c *check.C) {
	content := []byte(s.reg.Password())
	tmpfile, err := ioutil.TempFile("", "dockerpass")
	c.Assert(err, checker.IsNil)
	defer os.Remove(tmpfile.Name()) // clean up

	_, err = tmpfile.Write(content)
	c.Assert(err, checker.IsNil)

	err = tmpfile.Close()
	c.Assert(err, checker.IsNil)

	dockerCmd(c, "login", "-u", s.reg.Username(), "--password-file", tmpfile.Name(), privateRegistryURL)
}
