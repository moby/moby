package main

import (
	"os/exec"
	"strings"
	"testing"
)

// ensure docker info succeeds
func TestInfoEnsureSucceeds(t *testing.T) {
	versionCmd := exec.Command(dockerBinary, "info")
	out, exitCode, err := runCommandWithOutput(versionCmd)
	if err != nil || exitCode != 0 {
		t.Fatalf("failed to execute docker info: %s, %v", out, err)
	}

	// always shown fields
	stringsToCheck := []string{
		"ID:",
		"Containers:",
		"Images:",
		"Execution Driver:",
		"Logging Driver:",
		"Operating System:",
		"CPUs:",
		"Total Memory:",
		"Kernel Version:",
		"Storage Driver:"}

	for _, linePrefix := range stringsToCheck {
		if !strings.Contains(out, linePrefix) {
			t.Errorf("couldn't find string %v in output", linePrefix)
		}
	}

	logDone("info - verify that it works")
}
