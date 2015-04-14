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

func TestSearchCmdOptions(t *testing.T) {
	testRequires(t, Network)
	searchCmdhelp := exec.Command(dockerBinary, "search", "--help")
	out, exitCode, err := runCommandWithOutput(searchCmdhelp)
	if err != nil || exitCode != 0 {
		t.Fatalf("failed to get search help information: %s, %v", out, err)
	}

	if !strings.Contains(out, "Usage: docker search [OPTIONS] TERM") {
		t.Fatalf("failed to show docker search usage: %s, %v", out, err)
	}

	searchCmd := exec.Command(dockerBinary, "search", "busybox")
	outSearchCmd, exitCode, err := runCommandWithOutput(searchCmd)
	if err != nil || exitCode != 0 {
		t.Fatalf("failed to search on the central registry: %s, %v", outSearchCmd, err)
	}

	searchCmdautomated := exec.Command(dockerBinary, "search", "--automated=true", "busybox")
	outSearchCmdautomated, exitCode, err := runCommandWithOutput(searchCmdautomated) //The busybox is a busybox base image, not an AUTOMATED image.
	if err != nil || exitCode != 0 {
		t.Fatalf("failed to search with automated=true on the central registry: %s, %v", outSearchCmdautomated, err)
	}

	outSearchCmdautomatedSlice := strings.Split(outSearchCmdautomated, "\n")
	for i := range outSearchCmdautomatedSlice {
		if strings.HasPrefix(outSearchCmdautomatedSlice[i], "busybox ") {
			t.Fatalf("The busybox is not an AUTOMATED image: %s, %v", out, err)
		}
	}

	searchCmdStars := exec.Command(dockerBinary, "search", "-s=2", "busybox")
	outSearchCmdStars, exitCode, err := runCommandWithOutput(searchCmdStars)
	if err != nil || exitCode != 0 {
		t.Fatalf("failed to search with stars=2 on the central registry: %s, %v", outSearchCmdStars, err)
	}

	if strings.Count(outSearchCmdStars, "[OK]") > strings.Count(outSearchCmd, "[OK]") {
		t.Fatalf("The quantity of images with stars should be less than that of all images: %s, %v", outSearchCmdStars, err)
	}

	searchCmdOptions := exec.Command(dockerBinary, "search", "--stars=2", "--automated=true", "--no-trunc=true", "busybox")
	out, exitCode, err = runCommandWithOutput(searchCmdOptions)
	if err != nil || exitCode != 0 {
		t.Fatalf("failed to search with stars&automated&no-trunc options on the central registry: %s, %v", out, err)
	}

	logDone("search - have a try for search options.")
}
