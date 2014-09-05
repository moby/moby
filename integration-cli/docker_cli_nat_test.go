package main

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"testing"
)

func TestNetworkNat(t *testing.T) {
	iface, err := net.InterfaceByName("eth0")
	if err != nil {
		t.Skip("Test not running with `make test`. Interface eth0 not found: %s", err)
	}

	ifaceAddrs, err := iface.Addrs()
	if err != nil || len(ifaceAddrs) == 0 {
		t.Fatalf("Error retrieving addresses for eth0: %v (%d addresses)", err, len(ifaceAddrs))
	}

	ifaceIp, _, err := net.ParseCIDR(ifaceAddrs[0].String())
	if err != nil {
		t.Fatalf("Error retrieving the up for eth0: %s", err)
	}

	runCmd := exec.Command(dockerBinary, "run", "-dt", "-p", "8080:8080", "busybox", "nc", "-lp", "8080")
	out, _, err := runCommandWithOutput(runCmd)
	errorOut(err, t, fmt.Sprintf("run1 failed with errors: %v (%s)", err, out))

	cleanedContainerID := stripTrailingCharacters(out)

	runCmd = exec.Command(dockerBinary, "run", "busybox", "sh", "-c", fmt.Sprintf("echo hello world | nc -w 30 %s 8080", ifaceIp))
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, fmt.Sprintf("run2 failed with errors: %v (%s)", err, out))

	runCmd = exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, fmt.Sprintf("failed to retrieve logs for container: %v %v", cleanedContainerID, err))
	out = strings.Trim(out, "\r\n")

	if expected := "hello world"; out != expected {
		t.Fatalf("Unexpected output. Expected: %q, received: %q for iface %s", expected, out, ifaceIp)
	}

	killCmd := exec.Command(dockerBinary, "kill", cleanedContainerID)
	out, _, err = runCommandWithOutput(killCmd)
	errorOut(err, t, fmt.Sprintf("failed to kill container: %v %v", out, err))
	deleteAllContainers()

	logDone("network - make sure nat works through the host")
}
