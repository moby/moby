package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This is a heisen-test.  Because the created timestamp of images and the behavior of
// sort is not predictable it doesn't always fail.
func TestBuildHistory(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestBuildHistory")
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testbuildhistory", ".")

	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))
	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	out, exitCode, err = runCommandWithOutput(exec.Command(dockerBinary, "history", "testbuildhistory"))
	errorOut(err, t, fmt.Sprintf("image history failed: %v %v", out, err))
	if err != nil || exitCode != 0 {
		t.Fatal("failed to get image history")
	}

	actual_values := strings.Split(out, "\n")[1:27]
	expected_values := [26]string{"Z", "Y", "X", "W", "V", "U", "T", "S", "R", "Q", "P", "O", "N", "M", "L", "K", "J", "I", "H", "G", "F", "E", "D", "C", "B", "A"}

	for i := 0; i < 26; i++ {
		echo_value := fmt.Sprintf("echo \"%s\"", expected_values[i])
		actual_value := actual_values[i]

		if !strings.Contains(actual_value, echo_value) {
			t.Fatalf("Expected layer \"%s\", but was: %s", expected_values[i], actual_value)
		}
	}

	deleteImages("testbuildhistory")
}

func TestHistoryExistentImage(t *testing.T) {
	historyCmd := exec.Command(dockerBinary, "history", "busybox")
	_, exitCode, err := runCommandWithOutput(historyCmd)
	if err != nil || exitCode != 0 {
		t.Fatal("failed to get image history")
	}
	logDone("history - history on existent image must not fail")
}

func TestHistoryNonExistentImage(t *testing.T) {
	historyCmd := exec.Command(dockerBinary, "history", "testHistoryNonExistentImage")
	_, exitCode, err := runCommandWithOutput(historyCmd)
	if err == nil || exitCode == 0 {
		t.Fatal("history on a non-existent image didn't result in a non-zero exit status")
	}
	logDone("history - history on non-existent image must fail")
}
