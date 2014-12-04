package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

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
	out, _, _ = dockerCmd(t, "run", "-d", "--name", "container1", "busybox", "sleep", "10")
	idA := stripTrailingCharacters(out)
	out, _, _ = dockerCmd(t, "run", "-d", "--name", "container2", "busybox", "sleep", "10")
	idB := stripTrailingCharacters(out)
	dockerCmd(t, "run", "--rm", "--link", "container1:alias1", "--link", "container2:alias2", "busybox", "sh", "-c", "ping -c 1 alias1 -W 1 && ping -c 1 alias2 -W 1")
	dockerCmd(t, "kill", idA)
	dockerCmd(t, "kill", idB)
	deleteAllContainers()

	logDone("links - ping linked container")
}

func TestLinksIpTablesRulesWhenLinkAndUnlink(t *testing.T) {
	dockerCmd(t, "run", "-d", "--name", "child", "--publish", "8080:80", "busybox", "sleep", "10")
	dockerCmd(t, "run", "-d", "--name", "parent", "--link", "child:http", "busybox", "sleep", "10")

	childIP := findContainerIP(t, "child")
	parentIP := findContainerIP(t, "parent")

	sourceRule := []string{"FORWARD", "-i", "docker0", "-o", "docker0", "-p", "tcp", "-s", childIP, "--sport", "80", "-d", parentIP, "-j", "ACCEPT"}
	destinationRule := []string{"FORWARD", "-i", "docker0", "-o", "docker0", "-p", "tcp", "-s", parentIP, "--dport", "80", "-d", childIP, "-j", "ACCEPT"}
	if !iptables.Exists(sourceRule...) || !iptables.Exists(destinationRule...) {
		t.Fatal("Iptables rules not found")
	}

	dockerCmd(t, "rm", "--link", "parent/http")
	if iptables.Exists(sourceRule...) || iptables.Exists(destinationRule...) {
		t.Fatal("Iptables rules should be removed when unlink")
	}

	dockerCmd(t, "kill", "child")
	dockerCmd(t, "kill", "parent")
	deleteAllContainers()

	logDone("link - verify iptables when link and unlink")
}

func TestLinksInspectLinksStarted(t *testing.T) {
	var (
		expected = map[string]struct{}{"/container1:/testinspectlink/alias1": {}, "/container2:/testinspectlink/alias2": {}}
		result   []string
	)
	defer deleteAllContainers()
	dockerCmd(t, "run", "-d", "--name", "container1", "busybox", "sleep", "10")
	dockerCmd(t, "run", "-d", "--name", "container2", "busybox", "sleep", "10")
	dockerCmd(t, "run", "-d", "--name", "testinspectlink", "--link", "container1:alias1", "--link", "container2:alias2", "busybox", "sleep", "10")
	links, err := inspectFieldJSON("testinspectlink", "HostConfig.Links")
	if err != nil {
		t.Fatal(err)
	}

	err = unmarshalJSON([]byte(links), &result)
	if err != nil {
		t.Fatal(err)
	}

	output := convertSliceOfStringsToMap(result)

	equal := reflect.DeepEqual(output, expected)

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
	dockerCmd(t, "run", "-d", "--name", "container1", "busybox", "sleep", "10")
	dockerCmd(t, "run", "-d", "--name", "container2", "busybox", "sleep", "10")
	dockerCmd(t, "run", "-d", "--name", "testinspectlink", "--link", "container1:alias1", "--link", "container2:alias2", "busybox", "true")
	links, err := inspectFieldJSON("testinspectlink", "HostConfig.Links")
	if err != nil {
		t.Fatal(err)
	}

	err = unmarshalJSON([]byte(links), &result)
	if err != nil {
		t.Fatal(err)
	}

	output := convertSliceOfStringsToMap(result)

	equal := reflect.DeepEqual(output, expected)

	if !equal {
		t.Fatalf("Links %s, but expected %s", result, expected)
	}

	logDone("link - links in stopped container inspect")
}

func TestLinksNotStartedParentNotFail(t *testing.T) {
	defer deleteAllContainers()
	runCmd := exec.Command(dockerBinary, "create", "--name=first", "busybox", "top")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	runCmd = exec.Command(dockerBinary, "create", "--name=second", "--link=first:first", "busybox", "top")
	out, _, _, err = runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	runCmd = exec.Command(dockerBinary, "start", "first")
	out, _, _, err = runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	logDone("link - container start not failing on updating stopped parent links")
}

func TestLinksHostsFilesInject(t *testing.T) {
	defer deleteAllContainers()

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-itd", "--name", "one", "busybox", "top"))
	if err != nil {
		t.Fatal(err, out)
	}

	idOne := strings.TrimSpace(out)

	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "run", "-itd", "--name", "two", "--link", "one:onetwo", "busybox", "top"))
	if err != nil {
		t.Fatal(err, out)
	}

	idTwo := strings.TrimSpace(out)

	time.Sleep(1 * time.Second)

	contentOne, err := readContainerFile(idOne, "hosts")
	if err != nil {
		t.Fatal(err, string(contentOne))
	}

	contentTwo, err := readContainerFile(idTwo, "hosts")
	if err != nil {
		t.Fatal(err, string(contentTwo))
	}

	if !strings.Contains(string(contentTwo), "onetwo") {
		t.Fatal("Host is not present in updated hosts file", string(contentTwo))
	}

	logDone("link - ensure containers hosts files are updated with the link alias.")
}
