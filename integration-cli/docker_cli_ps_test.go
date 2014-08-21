package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestListContainers(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	errorOut(err, t, out)
	firstID := stripTrailingCharacters(out)

	runCmd = exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)
	secondID := stripTrailingCharacters(out)

	// not long running
	runCmd = exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)
	thirdID := stripTrailingCharacters(out)

	runCmd = exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)
	fourthID := stripTrailingCharacters(out)

	// make sure third one is not running
	runCmd = exec.Command(dockerBinary, "wait", thirdID)
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	// all
	runCmd = exec.Command(dockerBinary, "ps", "-a")
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	if !assertContainerList(out, []string{fourthID, thirdID, secondID, firstID}) {
		t.Error("Container list is not in the correct order")
	}

	// running
	runCmd = exec.Command(dockerBinary, "ps")
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	if !assertContainerList(out, []string{fourthID, secondID, firstID}) {
		t.Error("Container list is not in the correct order")
	}

	// from here all flag '-a' is ignored

	// limit
	runCmd = exec.Command(dockerBinary, "ps", "-n=2", "-a")
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)
	expected := []string{fourthID, thirdID}

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	runCmd = exec.Command(dockerBinary, "ps", "-n=2")
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	// since
	runCmd = exec.Command(dockerBinary, "ps", "--since", firstID, "-a")
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)
	expected = []string{fourthID, thirdID, secondID}

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	runCmd = exec.Command(dockerBinary, "ps", "--since", firstID)
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	// before
	runCmd = exec.Command(dockerBinary, "ps", "--before", thirdID, "-a")
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)
	expected = []string{secondID, firstID}

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	runCmd = exec.Command(dockerBinary, "ps", "--before", thirdID)
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	// since & before
	runCmd = exec.Command(dockerBinary, "ps", "--since", firstID, "--before", fourthID, "-a")
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)
	expected = []string{thirdID, secondID}

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	runCmd = exec.Command(dockerBinary, "ps", "--since", firstID, "--before", fourthID)
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)
	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	// since & limit
	runCmd = exec.Command(dockerBinary, "ps", "--since", firstID, "-n=2", "-a")
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)
	expected = []string{fourthID, thirdID}

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	runCmd = exec.Command(dockerBinary, "ps", "--since", firstID, "-n=2")
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	// before & limit
	runCmd = exec.Command(dockerBinary, "ps", "--before", fourthID, "-n=1", "-a")
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)
	expected = []string{thirdID}

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	runCmd = exec.Command(dockerBinary, "ps", "--before", fourthID, "-n=1")
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	// since & before & limit
	runCmd = exec.Command(dockerBinary, "ps", "--since", firstID, "--before", fourthID, "-n=1", "-a")
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)
	expected = []string{thirdID}

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	runCmd = exec.Command(dockerBinary, "ps", "--since", firstID, "--before", fourthID, "-n=1")
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	deleteAllContainers()

	logDone("ps - test ps options")
}

func assertContainerList(out string, expected []string) bool {
	lines := strings.Split(strings.Trim(out, "\n "), "\n")
	if len(lines)-1 != len(expected) {
		return false
	}

	containerIdIndex := strings.Index(lines[0], "CONTAINER ID")
	for i := 0; i < len(expected); i++ {
		foundID := lines[i+1][containerIdIndex : containerIdIndex+12]
		if foundID != expected[i][:12] {
			return false
		}
	}

	return true
}
