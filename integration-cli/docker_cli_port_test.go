package main

import (
	"net"
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
	firstID := strings.TrimSpace(out)

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
	ID := strings.TrimSpace(out)

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
	ID = strings.TrimSpace(out)

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

func TestPortHostBinding(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "-d", "-p", "9876:80", "busybox",
		"nc", "-l", "-p", "80")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	firstID := strings.TrimSpace(out)

	runCmd = exec.Command(dockerBinary, "port", firstID, "80")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if !assertPortList(t, out, []string{"0.0.0.0:9876"}) {
		t.Error("Port list is not correct")
	}

	runCmd = exec.Command(dockerBinary, "run", "--net=host", "busybox",
		"nc", "localhost", "9876")
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "rm", "-f", firstID)
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "run", "--net=host", "busybox",
		"nc", "localhost", "9876")
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		t.Error("Port is still bound after the Container is removed")
	}
	logDone("port - test host binding done")
}

func TestPortExposeHostBinding(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "-d", "-P", "--expose", "80", "busybox",
		"nc", "-l", "-p", "80")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	firstID := strings.TrimSpace(out)

	runCmd = exec.Command(dockerBinary, "port", firstID, "80")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	_, exposedPort, err := net.SplitHostPort(out)

	if err != nil {
		t.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "run", "--net=host", "busybox",
		"nc", "localhost", strings.TrimSpace(exposedPort))
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "rm", "-f", firstID)
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "run", "--net=host", "busybox",
		"nc", "localhost", strings.TrimSpace(exposedPort))
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		t.Error("Port is still bound after the Container is removed")
	}
	logDone("port - test port expose done")
}
