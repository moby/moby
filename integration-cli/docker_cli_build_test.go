package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
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

	go deleteImages("foobuildsixtysteps")

	logDone("build - build an image with sixty build steps")
}

// Test regression for gh#3960, gh#4848
func TestBuildAddRelative(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestBuildAddRelative")
	buildCmd := exec.Command(dockerBinary, "build", "-t", "foobuildaddrelative", ".")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	go deleteImages("foobuiladdrelative")

	logDone("build - build an image with sixty build steps")
}

// TODO: TestCaching

// TODO: TestADDCacheInvalidation
