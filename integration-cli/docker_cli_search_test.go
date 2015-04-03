package main

import (
	"os/exec"
	"strings"
	"testing"
)

// search for repos named  "registry" on the central registry
func TestSearchOnCentralRegistry(t *testing.T) {
	testRequires(t, Network)
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

func TestSearchStarsOptionWithWrongParameter(t *testing.T) {
	searchCmdStarsChars := exec.Command(dockerBinary, "search", "--stars=a", "busybox")
	out, exitCode, err := runCommandWithOutput(searchCmdStarsChars)
	if err == nil || exitCode == 0 {
		t.Fatalf("Should not get right information: %s, %v", out, err)
	}

	if !strings.Contains(out, "invalid value") {
		t.Fatal("couldn't find the invalid value warning")
	}

	searchCmdStarsNegativeNumber := exec.Command(dockerBinary, "search", "-s=-1", "busybox")
	out, exitCode, err = runCommandWithOutput(searchCmdStarsNegativeNumber)
	if err == nil || exitCode == 0 {
		t.Fatalf("Should not get right information: %s, %v", out, err)
	}

	if !strings.Contains(out, "invalid value") {
		t.Fatal("couldn't find the invalid value warning")
	}

	logDone("search - Verify search with wrong parameter.")
}
