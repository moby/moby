package main

import (
	"os/exec"
	"strings"
	"testing"
)

// ensure docker version works
func TestVersionEnsureSucceeds(t *testing.T) {
	versionCmd := exec.Command(dockerBinary, "version")
	out, _, err := runCommandWithOutput(versionCmd)
	if err != nil {
		t.Fatalf("failed to execute docker version: %s, %v", out, err)
	}

	stringsToCheck := []string{
		"Client version:",
		"Client API version:",
		"Go version (client):",
		"Git commit (client):",
		"OS/Arch (client):",
		"Server version:",
		"Server API version:",
		"Go version (server):",
		"Git commit (server):",
		"OS/Arch (server):",
	}

	for _, linePrefix := range stringsToCheck {
		if !strings.Contains(out, linePrefix) {
			t.Errorf("couldn't find string %v in output", linePrefix)
		}
	}

	logDone("version - verify that it works and that the output is properly formatted")
}
