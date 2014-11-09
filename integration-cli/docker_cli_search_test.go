package main

import (
	"os/exec"
	"strings"
	"testing"
)

// search for repos named  "registry" on the central registry
func TestSearchOnCentralRegistry(t *testing.T) {
	searchCmd := exec.Command(dockerBinary, "search", "busybox")
	out, exitCode, err := runCommandWithOutput(searchCmd)
	if err != nil || exitCode != 0 {
		t.Fatalf("failed to search on the central registry: %s, %v", out, err)
	}

	if !strings.Contains(out, "Busybox base image.") {
		t.Fatal("couldn't find any repository named (or containing) 'Busybox base image.'")
	}

	logDone("search - search for repositories named (or containing) 'Busybox base image.'")
}
