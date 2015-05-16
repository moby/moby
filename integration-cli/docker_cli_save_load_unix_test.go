// +build !windows

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/docker/docker/vendor/src/github.com/kr/pty"
	"github.com/go-check/check"
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

	saveCmdTemplate := `%v save %v > /tmp/foobar-save-load-test.tar`
	saveCmdFinal := fmt.Sprintf(saveCmdTemplate, dockerBinary, repoName)
	saveCmd := exec.Command("bash", "-c", saveCmdFinal)
	if out, _, err = runCommandWithOutput(saveCmd); err != nil {
		c.Fatalf("failed to save repo: %s, %v", out, err)
	}

	deleteImages(repoName)

	loadCmdFinal := `cat /tmp/foobar-save-load-test.tar | docker load`
	loadCmd := exec.Command("bash", "-c", loadCmdFinal)
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

	os.Remove("/tmp/foobar-save-load-test.tar")

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
