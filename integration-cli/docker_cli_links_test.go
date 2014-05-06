package main

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestPingUnlinkedContainers(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "--rm", "busybox", "sh", "-c", "ping -c 1 alias1 -W 1 && ping -c 1 alias2 -W 1")
	exitCode, err := runCommand(runCmd)

	if exitCode == 0 {
		t.Fatal("run ping did not fail")
	} else if exitCode != 1 {
		errorOut(err, t, fmt.Sprintf("run ping failed with errors: %v", err))
	}
}

func TestPingLinkedContainers(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-d", "--name", "container1", "busybox", "sleep", "10")
	out, _, err := runCommandWithOutput(cmd)
	errorOut(err, t, fmt.Sprintf("run container1 failed with errors: %v", err))
	idA := stripTrailingCharacters(out)

	cmd = exec.Command(dockerBinary, "run", "-d", "--name", "container2", "busybox", "sleep", "10")
	out, _, err = runCommandWithOutput(cmd)
	errorOut(err, t, fmt.Sprintf("run container2 failed with errors: %v", err))
	idB := stripTrailingCharacters(out)

	cmd = exec.Command(dockerBinary, "run", "--rm", "--link", "container1:alias1", "--link", "container2:alias2", "busybox", "sh", "-c", "ping -c 1 alias1 -W 1 && ping -c 1 alias2 -W 1")
	out, _, err = runCommandWithOutput(cmd)
	fmt.Printf("OUT: %s", out)
	errorOut(err, t, fmt.Sprintf("run ping failed with errors: %v", err))

	cmd = exec.Command(dockerBinary, "kill", idA)
	_, err = runCommand(cmd)
	errorOut(err, t, fmt.Sprintf("failed to kill container1: %v", err))

	cmd = exec.Command(dockerBinary, "kill", idB)
	_, err = runCommand(cmd)
	errorOut(err, t, fmt.Sprintf("failed to kill container2: %v", err))

	deleteAllContainers()
}
