package main

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func startServerContainer(t *testing.T, proto string) string {
	cmd := []string{"run", "-dt", "-p", "8080:8080", "busybox", "nc", "-lp", "8080"}
	if proto == "udp" {
		cmd = append(cmd, "-u")
	}

	runCmd := exec.Command(dockerBinary, cmd...)
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	return stripTrailingCharacters(out)
}

func getExternalAddress(t *testing.T) net.IP {
	iface, err := net.InterfaceByName("eth0")
	if err != nil {
		t.Skip("Test not running with `make test`. Interface eth0 not found: %s", err)
	}

	ifaceAddrs, err := iface.Addrs()
	if err != nil || len(ifaceAddrs) == 0 {
		t.Fatalf("Error retrieving addresses for eth0: %v (%d addresses)", err, len(ifaceAddrs))
	}

	ifaceIP, _, err := net.ParseCIDR(ifaceAddrs[0].String())
	if err != nil {
		t.Fatalf("Error retrieving the up for eth0: %s", err)
	}

	return ifaceIP
}

func getContainerLogs(t *testing.T, containerID string) string {
	runCmd := exec.Command(dockerBinary, "logs", containerID)
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("failed to retrieve logs for container: %s, %v", out, err)
	}
	return strings.Trim(out, "\r\n")
}

func getContainerStatus(t *testing.T, containerID string) string {
	runCmd := exec.Command(dockerBinary, "inspect", "-f", "{{.State.Running}}", containerID)
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("failed to retrieve container status: %s, %v", out, err)
	}
	return strings.Trim(out, "\r\n")
}

func killContainer(t *testing.T, containerID string) {
	killCmd := exec.Command(dockerBinary, "kill", containerID)
	if out, _, err := runCommandWithOutput(killCmd); err != nil {
		t.Fatalf("failed to kill container: %s, %v", out, err)
	}
}

func TestNetworkTCPNat(t *testing.T) {
	srv := startServerContainer(t, "tcp")

	// Spawn a new container which connects to the server through the
	// interface address.
	endpoint := getExternalAddress(t)
	runCmd := exec.Command(dockerBinary, "run", "busybox", "sh", "-c", fmt.Sprintf("echo hello world | nc -w 30 %s 8080", endpoint))
	if _, _, err := runCommandWithOutput(runCmd); err != nil {
		t.Fatal(err)
	}

	result := getContainerLogs(t, srv)
	if expected := "hello world"; result != expected {
		t.Fatalf("Unexpected output. Expected: %q, received: %q", expected, result)
	}

	killContainer(t, srv)
	deleteAllContainers()

	logDone("network - make sure nat works in TCP through the host")
}

func TestNetworkLocalhostTCPNat(t *testing.T) {
	srv := startServerContainer(t, "tcp")
	for i := 0; i != 5; i++ {
		value := getContainerStatus(t, srv)
		if value == "true" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Attempt to connect from the host to the listening container.
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Fatalf("Failed to connect to container (%v)", err)
	}
	if _, err := conn.Write([]byte("hello world\n")); err != nil {
		t.Fatal(err)
	}
	conn.Close()

	result := getContainerLogs(t, srv)
	if expected := "hello world"; result != expected {
		t.Fatalf("Unexpected output. Expected: %q, received: %q", expected, result)
	}

	killContainer(t, srv)
	deleteAllContainers()

	logDone("network - make sure nat works in TCP through the host loopback")
}
