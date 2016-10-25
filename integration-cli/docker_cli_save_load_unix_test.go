// +build !windows

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
	"github.com/kr/pty"
)

// save a repo and try to load it using stdout
func (s *DockerSuite) TestSaveAndLoadRepoStdout(c *check.C) {
	name := "test-save-and-load-repo-stdout"
	dockerCmd(c, "run", "--name", name, "busybox", "true")

	repoName := "foobar-save-load-test"
	before, _ := dockerCmd(c, "commit", name, repoName)
	before = strings.TrimRight(before, "\n")

	tmpFile, err := ioutil.TempFile("", "foobar-save-load-test.tar")
	c.Assert(err, check.IsNil)
	defer os.Remove(tmpFile.Name())

	saveCmd := exec.Command(dockerBinary, "save", repoName)
	saveCmd.Stdout = tmpFile

	_, err = runCommand(saveCmd)
	c.Assert(err, check.IsNil)

	tmpFile, err = os.Open(tmpFile.Name())
	c.Assert(err, check.IsNil)
	defer tmpFile.Close()

	deleteImages(repoName)

	loadCmd := exec.Command(dockerBinary, "load")
	loadCmd.Stdin = tmpFile

	out, _, err := runCommandWithOutput(loadCmd)
	c.Assert(err, check.IsNil, check.Commentf(out))

	after := inspectField(c, repoName, "Id")
	after = strings.TrimRight(after, "\n")

	c.Assert(after, check.Equals, before) //inspect is not the same after a save / load

	deleteImages(repoName)

	pty, tty, err := pty.Open()
	c.Assert(err, check.IsNil)
	cmd := exec.Command(dockerBinary, "save", repoName)
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	c.Assert(cmd.Start(), check.IsNil)
	c.Assert(cmd.Wait(), check.NotNil) //did not break writing to a TTY

	buf := make([]byte, 1024)

	n, err := pty.Read(buf)
	c.Assert(err, check.IsNil) //could not read tty output
	c.Assert(string(buf[:n]), checker.Contains, "Cowardly refusing", check.Commentf("help output is not being yielded", out))
}

func (s *DockerSuite) TestSaveAndLoadWithProgressBar(c *check.C) {
	name := "test-load"
	_, err := buildImage(name, `
	FROM busybox
	RUN touch aa
	`, true)
	c.Assert(err, check.IsNil)

	tmptar := name + ".tar"
	dockerCmd(c, "save", "-o", tmptar, name)
	defer os.Remove(tmptar)

	dockerCmd(c, "rmi", name)
	dockerCmd(c, "tag", "busybox", name)
	out, _ := dockerCmd(c, "load", "-i", tmptar)
	expected := fmt.Sprintf("The image %s:latest already exists, renaming the old one with ID", name)
	c.Assert(out, checker.Contains, expected)
}
