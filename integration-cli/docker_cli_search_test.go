package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// search for repos named  "registry" on the central registry
func TestSearchOnCentralRegistry(t *testing.T) {
	searchCmd := exec.Command(dockerBinary)
	out, exitCode, err := runCommandWithOutput(searchCmd)
	errorOut(err, t, fmt.Sprintf("encountered error while searching: %v", err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to search on the central registry")
	}

	if !strings.Contains(out, "registry") {
		t.Fatal("couldn't find any repository named (or containing) 'registry'")
	}

	logDone("search - search for repositories named (or containing) 'registry'")
}
