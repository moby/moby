package main

import (
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestNetworkNat(c *check.C) {
	testRequires(c, SameHostDaemon, NativeExecDriver)

	iface, err := net.InterfaceByName("eth0")
	if err != nil {
		c.Skip(fmt.Sprintf("Test not running with `make test`. Interface eth0 not found: %v", err))
	}

	ifaceAddrs, err := iface.Addrs()
	if err != nil || len(ifaceAddrs) == 0 {
		c.Fatalf("Error retrieving addresses for eth0: %v (%d addresses)", err, len(ifaceAddrs))
	}

	ifaceIP, _, err := net.ParseCIDR(ifaceAddrs[0].String())
	if err != nil {
		c.Fatalf("Error retrieving the up for eth0: %s", err)
	}

	runCmd := exec.Command(dockerBinary, "run", "-dt", "-p", "8080:8080", "busybox", "nc", "-lp", "8080")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)

	runCmd = exec.Command(dockerBinary, "run", "busybox", "sh", "-c", fmt.Sprintf("echo hello world | nc -w 30 %s 8080", ifaceIP))
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf("failed to retrieve logs for container: %s, %v", out, err)
	}

	out = strings.Trim(out, "\r\n")

	if expected := "hello world"; out != expected {
		c.Fatalf("Unexpected output. Expected: %q, received: %q for iface %s", expected, out, ifaceIP)
	}

	killCmd := exec.Command(dockerBinary, "kill", cleanedContainerID)
	if out, _, err = runCommandWithOutput(killCmd); err != nil {
		c.Fatalf("failed to kill container: %s, %v", out, err)
	}

}
