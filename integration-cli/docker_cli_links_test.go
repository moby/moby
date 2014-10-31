package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/iptables"
)

func TestLinksEtcHostsRegularFile(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "--net=host", "busybox", "ls", "-la", "/etc/hosts")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if !strings.HasPrefix(out, "-") {
		t.Errorf("/etc/hosts should be a regular file")
	}

	deleteAllContainers()

	logDone("link - /etc/hosts is a regular file")
}

func TestLinksEtcHostsContentMatch(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "--net=host", "busybox", "cat", "/etc/hosts")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	hosts, err := ioutil.ReadFile("/etc/hosts")
	if os.IsNotExist(err) {
		t.Skip("/etc/hosts does not exist, skip this test")
	}

	if out != string(hosts) {
		t.Errorf("container")
	}

	deleteAllContainers()

	logDone("link - /etc/hosts matches hosts copy")
}

func TestLinksPingUnlinkedContainers(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "--rm", "busybox", "sh", "-c", "ping -c 1 alias1 -W 1 && ping -c 1 alias2 -W 1")
	exitCode, err := runCommand(runCmd)

	if exitCode == 0 {
		t.Fatal("run ping did not fail")
	} else if exitCode != 1 {
		t.Fatalf("run ping failed with errors: %v", err)
	}

	logDone("links - ping unlinked container")
}

func TestLinksPingLinkedContainers(t *testing.T) {
	var out string
	out, _, _ = cmd(t, "run", "-d", "--name", "container1", "busybox", "sleep", "10")
	idA := stripTrailingCharacters(out)
	out, _, _ = cmd(t, "run", "-d", "--name", "container2", "busybox", "sleep", "10")
	idB := stripTrailingCharacters(out)
	cmd(t, "run", "--rm", "--link", "container1:alias1", "--link", "container2:alias2", "busybox", "sh", "-c", "ping -c 1 alias1 -W 1 && ping -c 1 alias2 -W 1")
	cmd(t, "kill", idA)
	cmd(t, "kill", idB)
	deleteAllContainers()

	logDone("links - ping linked container")
}

func TestLinksIpTablesRulesWhenLinkAndUnlink(t *testing.T) {
	cmd(t, "run", "-d", "--name", "child", "--publish", "8080:80", "busybox", "sleep", "10")
	cmd(t, "run", "-d", "--name", "parent", "--link", "child:http", "busybox", "sleep", "10")

	childIP := findContainerIP(t, "child")
	parentIP := findContainerIP(t, "parent")

	sourceRule := []string{"FORWARD", "-i", "docker0", "-o", "docker0", "-p", "tcp", "-s", childIP, "--sport", "80", "-d", parentIP, "-j", "ACCEPT"}
	destinationRule := []string{"FORWARD", "-i", "docker0", "-o", "docker0", "-p", "tcp", "-s", parentIP, "--dport", "80", "-d", childIP, "-j", "ACCEPT"}
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

func TestLinksInspectLinksStarted(t *testing.T) {
	var (
		expected = map[string]struct{}{"/container1:/testinspectlink/alias1": {}, "/container2:/testinspectlink/alias2": {}}
		result   []string
	)
	defer deleteAllContainers()
	cmd(t, "run", "-d", "--name", "container1", "busybox", "sleep", "10")
	cmd(t, "run", "-d", "--name", "container2", "busybox", "sleep", "10")
	cmd(t, "run", "-d", "--name", "testinspectlink", "--link", "container1:alias1", "--link", "container2:alias2", "busybox", "sleep", "10")
	links, err := inspectFieldJSON("testinspectlink", "HostConfig.Links")
	if err != nil {
		t.Fatal(err)
	}

	err = unmarshalJSON([]byte(links), &result)
	if err != nil {
		t.Fatal(err)
	}

	output := convertSliceOfStringsToMap(result)

	equal := deepEqual(expected, output)

	if !equal {
		t.Fatalf("Links %s, expected %s", result, expected)
	}
	logDone("link - links in started container inspect")
}

func TestLinksInspectLinksStopped(t *testing.T) {
	var (
		expected = map[string]struct{}{"/container1:/testinspectlink/alias1": {}, "/container2:/testinspectlink/alias2": {}}
		result   []string
	)
	defer deleteAllContainers()
	cmd(t, "run", "-d", "--name", "container1", "busybox", "sleep", "10")
	cmd(t, "run", "-d", "--name", "container2", "busybox", "sleep", "10")
	cmd(t, "run", "-d", "--name", "testinspectlink", "--link", "container1:alias1", "--link", "container2:alias2", "busybox", "true")
	links, err := inspectFieldJSON("testinspectlink", "HostConfig.Links")
	if err != nil {
		t.Fatal(err)
	}

	err = unmarshalJSON([]byte(links), &result)
	if err != nil {
		t.Fatal(err)
	}

	output := convertSliceOfStringsToMap(result)

	equal := deepEqual(expected, output)

	if !equal {
		t.Fatalf("Links %s, but expected %s", result, expected)
	}

	logDone("link - links in stopped container inspect")
}

func TestLinksEnvVars(t *testing.T) {
	defer deleteAllContainers()

	// First setup some containers that we'll link to later
	cmd(t, "run", "-d", "--name", "c0", "busybox", "sleep", "10")
	cmd(t, "run", "-d", "--expose", "81", "--name", "c1", "busybox", "sleep", "10")
	cmd(t, "run", "-d", "--expose", "92", "--expose", "93/udp", "--name", "c2", "busybox", "sleep", "10")

	// We're not going to check the full list of env vars set, the unit
	// testcases should already cover those.  Instead, we're just going to
	// check to make sure the top-level env vars are there - meaning
	// ones like DOCKER_LINKS, *_PORTS and *_NAME

	// First make sure DOCKER_LINKS isn't set when we're not linked
	out, _, _ := cmd(t, "exec", "c1", "env")
	if strings.Contains(out, "DOCKER_LINKS") {
		t.Fatal("DOCKER_LINKS was not supposed to be set: %s", out)
	}

	// Now do some linking and check the env list each time

	// Linking to a container w/o any exposed ports
	out, _, _ = cmd(t, "run", "--link", "c0:c0", "--name", "p1", "busybox", "env")
	if !strings.Contains(out, "DOCKER_LINKS=c0") ||
		!strings.Contains(out, "C0_NAME=/p1/c0") ||
		strings.Contains(out, "C0_PORTS") {
		t.Fatal("P1 - Unexpected output: %s", out)
	}

	// Link to container with one exposed port
	out, _, _ = cmd(t, "run", "--link", "c1:c1", "--name", "p2", "busybox", "env")
	if !strings.Contains(out, "DOCKER_LINKS=c1") ||
		!strings.Contains(out, "C1_NAME=/p2/c1") ||
		!strings.Contains(out, "C1_PORTS=81/tcp") {
		t.Fatal("P2 - Unexpected output: %s", out)
	}

	// Link to container with two exposed ports
	out, _, _ = cmd(t, "run", "--link", "c2:c2", "--name", "p3", "busybox", "env")
	if !strings.Contains(out, "DOCKER_LINKS=c2") ||
		!strings.Contains(out, "C2_NAME=/p3/c2") ||
		!strings.Contains(out, "C2_PORTS=92/tcp 93/udp") {
		t.Fatal("P3 - Unexpected output: %s", out)
	}

	// Link to all containers
	out, _, _ = cmd(t, "run", "--link", "c0:c0", "--link", "c1:c1", "--link", "c2:c2", "--name", "p4", "busybox", "env")
	if !strings.Contains(out, "DOCKER_LINKS=c0 c1 c2") ||
		!strings.Contains(out, "C0_NAME=/p4/c0") ||
		!strings.Contains(out, "C1_NAME=/p4/c1") ||
		!strings.Contains(out, "C2_NAME=/p4/c2") ||
		strings.Contains(out, "C0_PORTS") ||
		!strings.Contains(out, "C1_PORTS=81/tcp") ||
		!strings.Contains(out, "C2_PORTS=92/tcp 93/udp") {
		t.Fatal("P4 - Unexpected output: %s", out)
	}

	logDone("link - verify env vars")
}
