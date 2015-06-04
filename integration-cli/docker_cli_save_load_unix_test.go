// +build !windows

package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/go-check/check"
	"github.com/kr/pty"
)

// save a repo and try to load it using stdout
func (s *DockerSuite) TestSaveAndLoadRepoStdout(c *check.C) {
	name := "test-save-and-load-repo-stdout"
	runCmd := exec.Command(dockerBinary, "run", "--name", name, "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf("failed to create a container: %s, %v", out, err)
	}

	repoName := "foobar-save-load-test"

	commitCmd := exec.Command(dockerBinary, "commit", name, repoName)
	if out, _, err = runCommandWithOutput(commitCmd); err != nil {
		c.Fatalf("failed to commit container: %s, %v", out, err)
	}

	inspectCmd := exec.Command(dockerBinary, "inspect", repoName)
	before, _, err := runCommandWithOutput(inspectCmd)
	if err != nil {
		c.Fatalf("the repo should exist before saving it: %s, %v", before, err)
	}

	tmpFile, err := ioutil.TempFile("", "foobar-save-load-test.tar")
	c.Assert(err, check.IsNil)
	defer os.Remove(tmpFile.Name())

	saveCmd := exec.Command(dockerBinary, "save", repoName)
	saveCmd.Stdout = tmpFile

	if _, err = runCommand(saveCmd); err != nil {
		c.Fatalf("failed to save repo: %v", err)
	}

	tmpFile, err = os.Open(tmpFile.Name())
	c.Assert(err, check.IsNil)

	deleteImages(repoName)

	loadCmd := exec.Command(dockerBinary, "load")
	loadCmd.Stdin = tmpFile

	if out, _, err = runCommandWithOutput(loadCmd); err != nil {
		c.Fatalf("failed to load repo: %s, %v", out, err)
	}

	inspectCmd = exec.Command(dockerBinary, "inspect", repoName)
	after, _, err := runCommandWithOutput(inspectCmd)
	if err != nil {
		c.Fatalf("the repo should exist after loading it: %s %v", after, err)
	}

	if before != after {
		c.Fatalf("inspect is not the same after a save / load")
	}

	deleteImages(repoName)

	pty, tty, err := pty.Open()
	if err != nil {
		c.Fatalf("Could not open pty: %v", err)
	}
	cmd := exec.Command(dockerBinary, "save", repoName)
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	if err := cmd.Start(); err != nil {
		c.Fatalf("start err: %v", err)
	}
	if err := cmd.Wait(); err == nil {
		c.Fatal("did not break writing to a TTY")
	}

	buf := make([]byte, 1024)

	n, err := pty.Read(buf)
	if err != nil {
		c.Fatal("could not read tty output")
	}

	if !bytes.Contains(buf[:n], []byte("Cowardly refusing")) {
		c.Fatal("help output is not being yielded", out)
	}

}
