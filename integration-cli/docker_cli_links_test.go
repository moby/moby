package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/iptables"
)

func TestLinksEtcHostsRegularFile(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "--net=host", "busybox", "ls", "-la", "/etc/hosts")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, out)

	if !strings.HasPrefix(out, "-") {
		t.Errorf("/etc/hosts should be a regular file")
	}

	logDone("link - /etc/hosts is a regular file")
}

func TestLinksEtcHostsContentMatch(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "--net=host", "busybox", "cat", "/etc/hosts")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, out)

	hosts, err := ioutil.ReadFile("/etc/hosts")
	if os.IsNotExist(err) {
		t.Skip("/etc/hosts does not exist, skip this test")
	}

	if out != string(hosts) {
		t.Errorf("container")
	}

	logDone("link - /etc/hosts matches hosts copy")
}

func TestLinksPingUnlinkedContainers(t *testing.T) {
	defer deleteAllContainers()
	runCmd := exec.Command(dockerBinary, "run", "--rm", "busybox", "sh", "-c", "ping -c 1 alias1 -W 1 && ping -c 1 alias2 -W 1")
	exitCode, err := runCommand(runCmd)

	if exitCode == 0 {
		t.Fatal("run ping did not fail")
	} else if exitCode != 1 {
		errorOut(err, t, fmt.Sprintf("run ping failed with errors: %v", err))
	}

	logDone("links - ping unlinked container")
}

func TestLinksPingLinkedContainers(t *testing.T) {
	var out string
	defer deleteAllContainers()
	out, _, _ = cmd(t, "run", "-d", "--name", "container1", "busybox", "sleep", "10")
	idA := stripTrailingCharacters(out)
	out, _, _ = cmd(t, "run", "-d", "--name", "container2", "busybox", "sleep", "10")
	idB := stripTrailingCharacters(out)
	cmd(t, "run", "--rm", "--link", "container1:alias1", "--link", "container2:alias2", "busybox", "sh", "-c", "ping -c 1 alias1 -W 1 && ping -c 1 alias2 -W 1")
	cmd(t, "kill", idA)
	cmd(t, "kill", idB)

	logDone("links - ping linked container")
}

func TestLinksIpTablesRulesWhenLinkAndUnlink(t *testing.T) {
	defer deleteAllContainers()
	cmd(t, "run", "-d", "--name", "child", "--publish", "8080:80", "busybox", "sleep", "10")
	cmd(t, "run", "-d", "--name", "parent", "--link", "child:http", "busybox", "sleep", "10")

	childIp := findContainerIp(t, "child")
	parentIp := findContainerIp(t, "parent")

	sourceRule := []string{"FORWARD", "-i", "docker0", "-o", "docker0", "-p", "tcp", "-s", childIp, "--sport", "80", "-d", parentIp, "-j", "ACCEPT"}
	destinationRule := []string{"FORWARD", "-i", "docker0", "-o", "docker0", "-p", "tcp", "-s", parentIp, "--dport", "80", "-d", childIp, "-j", "ACCEPT"}
	if !iptables.Exists(sourceRule...) || !iptables.Exists(destinationRule...) {
		t.Fatal("Iptables rules not found")
	}

	cmd(t, "rm", "--link", "parent/child/http")
	if iptables.Exists(sourceRule...) || iptables.Exists(destinationRule...) {
		t.Fatal("Iptables rules should be removed when unlink")
	}

	cmd(t, "kill", "child")
	cmd(t, "kill", "parent")

	logDone("link - verify iptables when link and unlink")
}

func TestLinksInspectLinksStarted(t *testing.T) {
	var (
		expected = map[string]struct{}{"container1:alias1": {}, "container2:alias2": {}}
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
		expected = map[string]struct{}{"container1:alias1": {}, "container2:alias2": {}}
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

func TestLinkDoubleAlias(t *testing.T) {
	defer deleteAllContainers()
	cmd(t, "run", "-d", "--name", "one", "busybox", "top")
	cmd(t, "run", "-d", "--name", "two", "--link", "one:db", "busybox", "top")
	cmd(t, "run", "-d", "--name", "three", "--link", "one:db", "busybox", "top")
	logDone("link - two of the same aliases to the same container")
}

func TestLinkSameAliasFails(t *testing.T) {
	defer deleteAllContainers()
	_, err := runCommand(exec.Command(dockerBinary, "run", "-itd", "--name", "one", "busybox", "top"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = runCommand(exec.Command(dockerBinary, "run", "-itd", "--name", "two", "busybox", "top"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = runCommand(exec.Command(dockerBinary, "run", "-itd", "--name", "three", "--link", "two:one", "one:one", "busybox", "top"))
	if err == nil {
		t.Fatal("Two of the same alias were allowed")
	}

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--name", "four", "--link", "two:one", "--link", "one:two", "busybox", "sh", "-c", "cat /etc/hosts"))
	if err != nil {
		t.Fatal(err, out)
	}

	if !strings.Contains(out, "two") || !strings.Contains(out, "one") {
		t.Fatal("Hosts do not exist in linking container")
	}

	logDone("link - child/alias collisions")
}

func TestLinkAddLink(t *testing.T) {
	defer deleteAllContainers()
	cmd(t, "run", "-d", "--name", "one", "busybox", "top")
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "--name", "two", "busybox", "top"))
	if err != nil {
		t.Fatal(err, out)
	}

	cmd(t, "links", "add", "two", "one", "one2")

	f, err := os.Open(filepath.Join("/var/lib/docker/containers", strings.TrimSpace(out), "hosts"))
	if err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadAll(f)
	f.Close()

	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "one2") {
		t.Fatal("Content does not contain the new alias name in /etc/hosts", string(content))
	}

	logDone("link - docker links add")
}

func TestLinkRemoveLink(t *testing.T) {
	defer deleteAllContainers()

	cmd(t, "run", "-d", "--name", "one", "busybox", "top")
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "--name", "two", "busybox", "top"))
	if err != nil {
		t.Fatal(err, out)
	}

	cmd(t, "links", "add", "two", "one", "one2")

	f, err := os.Open(filepath.Join("/var/lib/docker/containers", strings.TrimSpace(out), "hosts"))
	if err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadAll(f)
	f.Close()

	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "one2") {
		t.Fatal("Content does not contain the new alias name in /etc/hosts", string(content))
	}

	cmd(t, "links", "remove", "two", "one", "one2")

	f, err = os.Open(filepath.Join("/var/lib/docker/containers", strings.TrimSpace(out), "hosts"))
	if err != nil {
		t.Fatal(err)
	}

	content, err = ioutil.ReadAll(f)
	f.Close()

	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(content), "one2") {
		t.Fatal("Content contains the removed alias name in /etc/hosts", string(content))
	}

	logDone("link - docker links remove")
}

func TestLinkRemoveLinkAddLink(t *testing.T) {
	defer deleteAllContainers()

	cmd(t, "run", "-d", "--name", "one", "busybox", "top")
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "--name", "two", "busybox", "top"))
	if err != nil {
		t.Fatal(err, out)
	}

	cmd(t, "links", "add", "two", "one", "one2")

	f, err := os.Open(filepath.Join("/var/lib/docker/containers", strings.TrimSpace(out), "hosts"))
	if err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadAll(f)
	f.Close()

	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "one2") {
		t.Fatal("Content does not contain the new alias name in /etc/hosts", string(content))
	}

	cmd(t, "links", "remove", "two", "one", "one2")

	f, err = os.Open(filepath.Join("/var/lib/docker/containers", strings.TrimSpace(out), "hosts"))
	if err != nil {
		t.Fatal(err)
	}

	content, err = ioutil.ReadAll(f)
	f.Close()

	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(content), "one2") {
		t.Fatal("Content contains the removed alias name in /etc/hosts", string(content))
	}

	cmd(t, "links", "add", "two", "one", "one2")

	f, err = os.Open(filepath.Join("/var/lib/docker/containers", strings.TrimSpace(out), "hosts"))
	if err != nil {
		t.Fatal(err)
	}

	content, err = ioutil.ReadAll(f)
	f.Close()

	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "one2") {
		t.Fatal("Content does not contain the new alias name in /etc/hosts", string(content))
	}

	logDone("link - docker links add, then remove, then add")
}

func TestLinkToStoppedContainer(t *testing.T) {
	defer deleteAllContainers()

	cmd(t, "run", "-d", "--name", "one", "busybox", "top")
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "--name", "two", "busybox", "top"))
	if err != nil {
		t.Fatal(err, out)
	}

	cmd(t, "stop", "one")
	cmd(t, "links", "add", "two", "one", "one2")

	f, err := os.Open(filepath.Join("/var/lib/docker/containers", strings.TrimSpace(out), "hosts"))
	if err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadAll(f)
	f.Close()

	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "one2") {
		t.Fatal("Content does not contain the new alias name in /etc/hosts", string(content))
	}

	logDone("link - link to stopped container")
}
