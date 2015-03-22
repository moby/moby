package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestImportDisplay(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal("failed to create a container", out, err)
	}
	cleanedContainerID := strings.TrimSpace(out)
	defer deleteContainer(cleanedContainerID)

	out, _, err = runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "export", cleanedContainerID),
		exec.Command(dockerBinary, "import", "-"),
	)
	if err != nil {
		t.Errorf("import failed with errors: %v, output: %q", err, out)
	}

	if n := strings.Count(out, "\n"); n != 1 {
		t.Fatalf("display is messed up: %d '\\n' instead of 1:\n%s", n, out)
	}
	image := strings.TrimSpace(out)
	defer deleteImages(image)

	runCmd = exec.Command(dockerBinary, "run", "--rm", image, "true")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal("failed to create a container", out, err)
	}

	if out != "" {
		t.Fatalf("command output should've been nothing, was %q", out)
	}

	logDone("import - display is fine, imported image runs")
}

func TestImportFromSaveStdout(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("failed to create a container: %v %v", out, err)
	}
	cleanedContainerID := strings.TrimSpace(out)
	defer deleteContainer(cleanedContainerID)

	repoName := "foobar-save-load-test-tar"

	commitCmd := exec.Command(dockerBinary, "commit", cleanedContainerID, repoName)
	out, _, err = runCommandWithOutput(commitCmd)
	if err != nil {
		t.Fatalf("failed to commit container: %v %v", out, err)
	}

	saveCommand := exec.Command(dockerBinary, "save", repoName)
	repoTarball, _, err := runCommandWithOutput(saveCommand)
	if err != nil {
		t.Fatalf("failed to save repo: %v %v", repoTarball, err)
	}
	deleteImages(repoName)

	importCmd := exec.Command(dockerBinary, "import", "-", "test:latest")
	importCmd.Stdin = strings.NewReader(repoTarball)
	out, _, err = runCommandWithOutput(importCmd)
	if err == nil {
		t.Fatalf("expected error, but succeeded with no error and output: %v", out)
	}

	logDone("import - importing previously saved image gives an error")
}
