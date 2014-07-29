package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// ensure docker info succeeds
func TestInfoEnsureSucceeds(t *testing.T) {
	versionCmd := exec.Command(dockerBinary, "info")
	out, exitCode, err := runCommandWithOutput(versionCmd)
	errorOut(err, t, fmt.Sprintf("encountered error while running docker info: %v", err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to execute docker info")
	}

	stringsToCheck := []string{
		"Client version:",
		"Client API version:",
		"Go version (client):",
		"Git commit (client):",
		"Server version:",
		"Server API version:",
		"Go version (server):",
		"Git commit (server):",

		"Containers:",
		"Execution Driver:",
		"Kernel Version:",
	}

	for _, linePrefix := range stringsToCheck {
		if !strings.Contains(out, linePrefix) {
			t.Errorf("couldn't find string %v in output", linePrefix)
		}
	}

	logDone("info - verify that it works")
}
