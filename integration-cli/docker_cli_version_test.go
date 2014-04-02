package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// ensure docker version works
func TestVersionEnsureSucceeds(t *testing.T) {
	versionCmd := exec.Command(dockerBinary, "version")
	out, exitCode, err := runCommandWithOutput(versionCmd)
	errorOut(err, t, fmt.Sprintf("encountered error while running docker version: %v", err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to execute docker version")
	}

	stringsToCheck := []string{
		"Client version:",
		"Client API version:",
		"Go version (client):",
		"Git commit (client):",
		"Server version:",
		"Server API version:",
		"Git commit (server):",
		"Go version (server):",
		"Last stable version:",
	}

	for _, linePrefix := range stringsToCheck {
		if !strings.Contains(out, linePrefix) {
			t.Errorf("couldn't find string %v in output", linePrefix)
		}
	}

	logDone("version - verify that it works and that the output is properly formatted")
}
