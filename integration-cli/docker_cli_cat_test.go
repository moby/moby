package main

import (
	"os/exec"
	"strings"
	"testing"
)

// Check that specifying absolute path outputs something
func TestCatAbsPath(t *testing.T) {
	// Create container
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox",
		"sh", "-c", "echo -en 'hello world\n' > /testfile")
	stdout, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("run failed with errors: %v", err)
	}
	containerId := stripTrailingCharacters(stdout)
	exec.Command(dockerBinary, "wait", containerId).Run()

	// Run "docker cat <container> /testfile" and check its output
	catCmd := exec.Command(dockerBinary, "cat", containerId, "/testfile")
	stdout, stderr, exitCode, err := runCommandWithStdoutStderr(catCmd)
	if err != nil {
		t.Fatalf("cat failed with errors: %v", err)
	}
	if stdout != "hello world\n" {
		t.Errorf("stdout does not match expected value: %v", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr does not match expected value: %v", stderr)
	}
	if exitCode != 0 {
		t.Errorf("exitCode is %d, should be 0", exitCode)
	}

	logDone("cat - print a single file")
}

// Check that relative paths are not accepted
func TestCatRelPath(t *testing.T) {
	// Create container
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox")
	stdout, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("run failed with errors: %v", err)
	}
	containerId := stripTrailingCharacters(stdout)
	exec.Command(dockerBinary, "wait", containerId).Run()

	// Run "docker cat <container> testfile" and check its output
	catCmd := exec.Command(dockerBinary, "cat", containerId, "testfile")
	stdout, stderr, exitCode, _ := runCommandWithStdoutStderr(catCmd)
	if stdout != "" {
		t.Errorf("stdout does not match expected value: %v", stdout)
	}
	if !strings.Contains(stderr, "File path must be absolute") {
		t.Errorf("stderr does not match expected value: %v", stderr)
	}
	if exitCode != 1 {
		t.Errorf("exitCode is %d, should be 1", exitCode)
	}

	logDone("cat - reject relative path")
}

// Check that specifying no paths outputs nothing
func TestCatNoPaths(t *testing.T) {
	// Create container
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox")
	stdout, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("run failed with errors: %v", err)
	}
	containerId := stripTrailingCharacters(stdout)
	exec.Command(dockerBinary, "wait", containerId).Run()

	// Run "docker cat <container>" and check its output
	catCmd := exec.Command(dockerBinary, "cat", containerId)
	stdout, stderr, exitCode, err := runCommandWithStdoutStderr(catCmd)
	if stdout != "" {
		t.Errorf("stdout does not match expected value: %v", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr does not match expected value: %v", stderr)
	}
	if exitCode != 0 {
		t.Errorf("exitCode is %d, should be 0", exitCode)
	}
	if err != nil {
		t.Fatalf("cat failed with errors: %v", err)
	}

	logDone("cat - no paths no output")
}

// Check that specifying multiple paths outputs contents of all files
func TestCatManyPaths(t *testing.T) {
	// Create container
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox",
		"sh", "-c", "echo -en 'hello world\n' > /testfile; echo -en 'hello universe\n' > /testfile2")
	stdout, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("run failed with errors: %v", err)
	}
	containerId := stripTrailingCharacters(stdout)
	exec.Command(dockerBinary, "wait", containerId).Run()

	// Run "docker cat <container> /testfile /testfile2" and check its output
	catCmd := exec.Command(dockerBinary, "cat", containerId, "/testfile", "/testfile2")
	stdout, stderr, exitCode, err := runCommandWithStdoutStderr(catCmd)
	if stdout != "hello world\nhello universe\n" {
		t.Errorf("stdout does not match expected value: %v", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr does not match expected value: %v", stderr)
	}
	if exitCode != 0 {
		t.Errorf("exitCode is %d, should be 0", exitCode)
	}
	if err != nil {
		t.Fatalf("cat failed with errors: %v", err)
	}

	logDone("cat - print multiple files")
}
