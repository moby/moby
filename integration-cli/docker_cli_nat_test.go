package main

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"

	"github.com/go-check/check"
)

func startServerContainer(c *check.C, proto string, port int) string {
	cmd := []string{"-d", "-p", fmt.Sprintf("%d:%d", port, port), "busybox", "nc", "-lp", strconv.Itoa(port)}
	if proto == "udp" {
		cmd = append(cmd, "-u")
	}

	name := "server"
	if err := waitForContainer(name, cmd...); err != nil {
		c.Fatalf("Failed to launch server container: %v", err)
	}
	return name
}

func getExternalAddress(c *check.C) net.IP {
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

	return ifaceIP
}

func getContainerLogs(c *check.C, containerID string) string {
	runCmd := exec.Command(dockerBinary, "logs", containerID)
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
	return strings.Trim(out, "\r\n")
}

func getContainerStatus(c *check.C, containerID string) string {
	runCmd := exec.Command(dockerBinary, "inspect", "-f", "{{.State.Running}}", containerID)
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
	return strings.Trim(out, "\r\n")
}

func (s *DockerSuite) TestNetworkNat(c *check.C) {
	testRequires(c, SameHostDaemon, NativeExecDriver)
	defer deleteAllContainers()

	srv := startServerContainer(c, "tcp", 8080)

	// Spawn a new container which connects to the server through the
	// interface address.
	endpoint := getExternalAddress(c)
	runCmd := exec.Command(dockerBinary, "run", "busybox", "sh", "-c", fmt.Sprintf("echo hello world | nc -w 30 %s 8080", endpoint))
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		c.Fatalf("Failed to connect to server: %v (output: %q)", err, string(out))
	}

	result := getContainerLogs(c, srv)
	if expected := "hello world"; result != expected {
		c.Fatalf("Unexpected output. Expected: %q, received: %q", expected, result)
	}
}

func (s *DockerSuite) TestNetworkLocalhostTCPNat(c *check.C) {
	testRequires(c, SameHostDaemon, NativeExecDriver)
	defer deleteAllContainers()

	srv := startServerContainer(c, "tcp", 8081)

	// Attempt to connect from the host to the listening container.
	conn, err := net.Dial("tcp", "localhost:8081")
	if err != nil {
		c.Fatalf("Failed to connect to container (%v)", err)
	}
	if _, err := conn.Write([]byte("hello world\n")); err != nil {
		c.Fatal(err)
	}
	conn.Close()

	result := getContainerLogs(c, srv)
	if expected := "hello world"; result != expected {
		c.Fatalf("Unexpected output. Expected: %q, received: %q", expected, result)
	}
}
