// +build !windows

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/docker/docker/vendor/src/github.com/kr/pty"
)

// save a repo and try to load it using stdout
func TestSaveAndLoadRepoStdout(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("failed to create a container: %s, %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	repoName := "foobar-save-load-test"

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedContainerID)
	if out, _, err = runCommandWithOutput(inspectCmd); err != nil {
		t.Fatalf("output should've been a container id: %s, %v", out, err)
	}

	commitCmd := exec.Command(dockerBinary, "commit", cleanedContainerID, repoName)
	if out, _, err = runCommandWithOutput(commitCmd); err != nil {
		t.Fatalf("failed to commit container: %s, %v", out, err)
	}

	inspectCmd = exec.Command(dockerBinary, "inspect", repoName)
	before, _, err := runCommandWithOutput(inspectCmd)
	if err != nil {
		t.Fatalf("the repo should exist before saving it: %s, %v", before, err)
	}

	saveCmdTemplate := `%v save %v > /tmp/foobar-save-load-test.tar`
	saveCmdFinal := fmt.Sprintf(saveCmdTemplate, dockerBinary, repoName)
	saveCmd := exec.Command("bash", "-c", saveCmdFinal)
	if out, _, err = runCommandWithOutput(saveCmd); err != nil {
		t.Fatalf("failed to save repo: %s, %v", out, err)
	}

	deleteImages(repoName)

	loadCmdFinal := `cat /tmp/foobar-save-load-test.tar | docker load`
	loadCmd := exec.Command("bash", "-c", loadCmdFinal)
	if out, _, err = runCommandWithOutput(loadCmd); err != nil {
		t.Fatalf("failed to load repo: %s, %v", out, err)
	}

	inspectCmd = exec.Command(dockerBinary, "inspect", repoName)
	after, _, err := runCommandWithOutput(inspectCmd)
	if err != nil {
		t.Fatalf("the repo should exist after loading it: %s %v", after, err)
	}

	if before != after {
		t.Fatalf("inspect is not the same after a save / load")
	}

	deleteContainer(cleanedContainerID)
	deleteImages(repoName)

	os.Remove("/tmp/foobar-save-load-test.tar")

	logDone("save - save/load a repo using stdout")

	pty, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("Could not open pty: %v", err)
	}
	cmd := exec.Command(dockerBinary, "save", repoName)
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	if err := cmd.Start(); err != nil {
		t.Fatalf("start err: %v", err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("did not break writing to a TTY")
	}

	buf := make([]byte, 1024)

	n, err := pty.Read(buf)
	if err != nil {
		t.Fatal("could not read tty output")
	}

	if !bytes.Contains(buf[:n], []byte("Cowardly refusing")) {
		t.Fatal("help output is not being yielded", out)
	}

	logDone("save - do not save to a tty")
}
