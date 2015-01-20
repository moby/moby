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
		t.Skipf("Test not running with `make test`. Interface eth0 not found: %s", err)
	}

	ifaceAddrs, err := iface.Addrs()
	if err != nil || len(ifaceAddrs) == 0 {
		t.Fatalf("Error retrieving addresses for eth0: %v (%d addresses)", err, len(ifaceAddrs))
	}

	ifaceIP, _, err := net.ParseCIDR(ifaceAddrs[0].String())
	if err != nil {
		t.Fatalf("Error retrieving the up for eth0: %s", err)
	}

	runCmd := exec.Command(dockerBinary, "run", "-dt", "-p", "8080:8080", "busybox", "nc", "-lp", "8080")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	runCmd = exec.Command(dockerBinary, "run", "busybox", "sh", "-c", fmt.Sprintf("echo hello world | nc -w 30 %s 8080", ifaceIP))
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("failed to retrieve logs for container: %s, %v", out, err)
	}

	out = strings.Trim(out, "\r\n")

	if expected := "hello world"; out != expected {
		t.Fatalf("Unexpected output. Expected: %q, received: %q for iface %s", expected, out, ifaceIP)
	}

	killCmd := exec.Command(dockerBinary, "kill", cleanedContainerID)
	if out, _, err = runCommandWithOutput(killCmd); err != nil {
		t.Fatalf("failed to kill container: %s, %v", out, err)
	}
	deleteAllContainers()

	logDone("network - make sure nat works through the host")
}
