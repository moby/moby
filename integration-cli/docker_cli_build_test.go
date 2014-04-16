package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSixtySteps(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestBuildSixtySteps")
	buildCmd := exec.Command(dockerBinary, "build", "-t", "foobuildsixtysteps", ".")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("foobuildsixtysteps")

	logDone("build - build an image with sixty build steps")
}

func TestAddFileOwnership(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestAddFileOwnership")
	buildCmd := exec.Command(dockerBinary, "build", "-t", "fooaddfileownership", ".")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	runCmd := exec.Command(dockerBinary, "run", "-t", "fooaddfileownership", "ls", "-ld", "/test/")
	out, exitCode, err = runCommandWithOutput(runCmd)
	errorOut(err, t, fmt.Sprintf("run failed to : %v %v", out, err))

	if err != nil || exitCode != 0 {
		deleteImages("fooaddfileownership")
		t.Fatal("failed to run ls in the image")
	}

	if !strings.Contains(out, "nobody") {
		deleteImages("fooaddfileownership")
		t.Fatalf("Expected to find owner nobody: %s", out)
	}

	deleteImages("fooaddfileownership")

	logDone("build - build an image adding a file")
}

// TODO: TestCaching

// TODO: TestADDCacheInvalidation
