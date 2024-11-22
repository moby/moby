//go:build !windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/cli/build"
	"github.com/docker/docker/testutil"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
	"gotest.tools/v3/skip"
)

// save a repo and try to load it using stdout
func (s *DockerCLISaveLoadSuite) TestSaveAndLoadRepoStdout(c *testing.T) {
	name := "test-save-and-load-repo-stdout"
	cli.DockerCmd(c, "run", "--name", name, "busybox", "true")

	imgRepoName := "foobar-save-load-test"
	before := cli.DockerCmd(c, "commit", name, imgRepoName).Stdout()
	before = strings.TrimRight(before, "\n")

	tmpFile, err := os.CreateTemp("", "foobar-save-load-test.tar")
	assert.NilError(c, err)
	defer os.Remove(tmpFile.Name())

	icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "save", imgRepoName},
		Stdout:  tmpFile,
	}).Assert(c, icmd.Success)

	tmpFile, err = os.Open(tmpFile.Name())
	assert.NilError(c, err)
	defer tmpFile.Close()

	deleteImages(imgRepoName)

	icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "load"},
		Stdin:   tmpFile,
	}).Assert(c, icmd.Success)

	after := inspectField(c, imgRepoName, "Id")
	after = strings.TrimRight(after, "\n")

	assert.Equal(c, after, before, "inspect is not the same after a save / load")

	deleteImages(imgRepoName)

	p, tty, err := pty.Open()
	assert.NilError(c, err)
	cmd := exec.Command(dockerBinary, "save", imgRepoName)
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	assert.NilError(c, cmd.Start())
	assert.ErrorContains(c, cmd.Wait(), "", "did not break writing to a TTY")

	buf := make([]byte, 1024)

	n, err := p.Read(buf)
	assert.NilError(c, err, "could not read tty output")
	assert.Assert(c, strings.Contains(string(buf[:n]), "cowardly refusing"), "help output is not being yielded")
}

func (s *DockerCLISaveLoadSuite) TestSaveAndLoadWithProgressBar(c *testing.T) {
	// TODO(vvoland): https://github.com/moby/moby/issues/43910
	skip.If(c, testEnv.UsingSnapshotter(), "TODO: Not implemented yet")

	name := "test-load"
	buildImageSuccessfully(c, name, build.WithDockerfile(`FROM busybox
	RUN touch aa
	`))

	tmptar := name + ".tar"
	cli.DockerCmd(c, "save", "-o", tmptar, name)
	defer os.Remove(tmptar)

	cli.DockerCmd(c, "rmi", name)
	cli.DockerCmd(c, "tag", "busybox", name)
	out := cli.DockerCmd(c, "load", "-i", tmptar).Combined()
	expected := fmt.Sprintf("The image %s:latest already exists, renaming the old one with ID", name)
	assert.Assert(c, is.Contains(out, expected))
}

// fail because load didn't receive data from stdin
func (s *DockerCLISaveLoadSuite) TestLoadNoStdinFail(c *testing.T) {
	p, tty, err := pty.Open()
	assert.NilError(c, err)
	ctx, cancel := context.WithTimeout(testutil.GetContext(c), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, dockerBinary, "load")
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	assert.ErrorContains(c, cmd.Run(), "", "docker-load should fail")

	buf := make([]byte, 1024)

	n, err := p.Read(buf)
	assert.NilError(c, err) // could not read tty output
	assert.Assert(c, is.Contains(string(buf[:n]), "requested load from stdin, but stdin is empty"))
}
