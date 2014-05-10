package main

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/iptables"
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
	var out string
	out, _, _ = cmd(t, "run", "-d", "--name", "container1", "busybox", "sleep", "10")
	idA := stripTrailingCharacters(out)
	out, _, _ = cmd(t, "run", "-d", "--name", "container2", "busybox", "sleep", "10")
	idB := stripTrailingCharacters(out)
	cmd(t, "run", "--rm", "--link", "container1:alias1", "--link", "container2:alias2", "busybox", "sh", "-c", "ping -c 1 alias1 -W 1 && ping -c 1 alias2 -W 1")
	cmd(t, "kill", idA)
	cmd(t, "kill", idB)
	deleteAllContainers()
}

func TestIpTablesRulesWhenLinkAndUnlink(t *testing.T) {
	cmd(t, "run", "-d", "--name", "child", "--publish", "8080:80", "busybox", "sleep", "10")
	cmd(t, "run", "-d", "--name", "parent", "--link", "child:http", "busybox", "sleep", "10")

	childIp := findContainerIp(t, "child")
	parentIp := findContainerIp(t, "parent")

	sourceRule := []string{"FORWARD", "-i", "docker0", "-o", "docker0", "-p", "tcp", "-s", childIp, "--sport", "80", "-d", parentIp, "-j", "ACCEPT"}
	destinationRule := []string{"FORWARD", "-i", "docker0", "-o", "docker0", "-p", "tcp", "-s", parentIp, "--dport", "80", "-d", childIp, "-j", "ACCEPT"}
	if !iptables.Exists(sourceRule...) || !iptables.Exists(destinationRule...) {
		t.Fatal("Iptables rules not found")
	}

	cmd(t, "rm", "--link", "parent/http")
	if iptables.Exists(sourceRule...) || iptables.Exists(destinationRule...) {
		t.Fatal("Iptables rules should be removed when unlink")
	}

	cmd(t, "kill", "child")
	cmd(t, "kill", "parent")
	deleteAllContainers()

	logDone("link - verify iptables when link and unlink")
}
