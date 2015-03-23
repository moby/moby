package main

import (
	"os/exec"
	"sort"
	"strings"
	"testing"
)

func TestPortList(t *testing.T) {
	defer deleteAllContainers()

	// one port
	runCmd := exec.Command(dockerBinary, "run", "-d", "-p", "9876:80", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	firstID := stripTrailingCharacters(out)

	runCmd = exec.Command(dockerBinary, "port", firstID, "80")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if !assertPortList(t, out, []string{"0.0.0.0:9876"}) {
		t.Error("Port list is not correct")
	}

	runCmd = exec.Command(dockerBinary, "port", firstID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if !assertPortList(t, out, []string{"80/tcp -> 0.0.0.0:9876"}) {
		t.Error("Port list is not correct")
	}
	runCmd = exec.Command(dockerBinary, "rm", "-f", firstID)
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	// three port
	runCmd = exec.Command(dockerBinary, "run", "-d",
		"-p", "9876:80",
		"-p", "9877:81",
		"-p", "9878:82",
		"busybox", "top")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	ID := stripTrailingCharacters(out)

	runCmd = exec.Command(dockerBinary, "port", ID, "80")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if !assertPortList(t, out, []string{"0.0.0.0:9876"}) {
		t.Error("Port list is not correct")
	}

	runCmd = exec.Command(dockerBinary, "port", ID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if !assertPortList(t, out, []string{
		"80/tcp -> 0.0.0.0:9876",
		"81/tcp -> 0.0.0.0:9877",
		"82/tcp -> 0.0.0.0:9878"}) {
		t.Error("Port list is not correct")
	}
	runCmd = exec.Command(dockerBinary, "rm", "-f", ID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	// more and one port mapped to the same container port
	runCmd = exec.Command(dockerBinary, "run", "-d",
		"-p", "9876:80",
		"-p", "9999:80",
		"-p", "9877:81",
		"-p", "9878:82",
		"busybox", "top")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	ID = stripTrailingCharacters(out)

	runCmd = exec.Command(dockerBinary, "port", ID, "80")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if !assertPortList(t, out, []string{"0.0.0.0:9876", "0.0.0.0:9999"}) {
		t.Error("Port list is not correct")
	}

	runCmd = exec.Command(dockerBinary, "port", ID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if !assertPortList(t, out, []string{
		"80/tcp -> 0.0.0.0:9876",
		"80/tcp -> 0.0.0.0:9999",
		"81/tcp -> 0.0.0.0:9877",
		"82/tcp -> 0.0.0.0:9878"}) {
		t.Error("Port list is not correct\n", out)
	}
	runCmd = exec.Command(dockerBinary, "rm", "-f", ID)
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	logDone("port - test port list")
}

func assertPortList(t *testing.T, out string, expected []string) bool {
	//lines := strings.Split(out, "\n")
	lines := strings.Split(strings.Trim(out, "\n "), "\n")
	if len(lines) != len(expected) {
		t.Errorf("different size lists %s, %d, %d", out, len(lines), len(expected))
		return false
	}
	sort.Strings(lines)
	sort.Strings(expected)

	for i := 0; i < len(expected); i++ {
		if lines[i] != expected[i] {
			t.Error("|" + lines[i] + "!=" + expected[i] + "|")
			return false
		}
	}

	return true
}
