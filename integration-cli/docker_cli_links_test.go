package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/pkg/iptables"
)

func TestLinksEtcHostsRegularFile(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "--net=host", "busybox", "ls", "-la", "/etc/hosts")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if !strings.HasPrefix(out, "-") {
		t.Errorf("/etc/hosts should be a regular file")
	}
	logDone("link - /etc/hosts is a regular file")
}

func TestLinksEtcHostsContentMatch(t *testing.T) {
	testRequires(t, SameHostDaemon)
	defer deleteAllContainers()

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

// Test for appropriate error when calling --link with an invalid target container
func TestLinksInvalidContainerTarget(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "--link", "bogus:alias", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)

	if err == nil {
		t.Fatal("an invalid container target should produce an error")
	}
	if !strings.Contains(out, "Could not get container") {
		t.Fatal("error output expected 'Could not get container', but got %q instead; err: %v", out, err)
	}

	logDone("links - linking to non-existent container should not work")
}

func TestLinksPingLinkedContainers(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "-d", "--name", "container1", "--hostname", "fred", "busybox", "top")
	if _, err := runCommand(runCmd); err != nil {
		t.Fatal(err)
	}
	runCmd = exec.Command(dockerBinary, "run", "-d", "--name", "container2", "--hostname", "wilma", "busybox", "top")
	if _, err := runCommand(runCmd); err != nil {
		t.Fatal(err)
	}

	runArgs := []string{"run", "--rm", "--link", "container1:alias1", "--link", "container2:alias2", "busybox", "sh", "-c"}
	pingCmd := "ping -c 1 %s -W 1 && ping -c 1 %s -W 1"

	// test ping by alias, ping by name, and ping by hostname
	// 1. Ping by alias
	dockerCmd(t, append(runArgs, fmt.Sprintf(pingCmd, "alias1", "alias2"))...)
	// 2. Ping by container name
	dockerCmd(t, append(runArgs, fmt.Sprintf(pingCmd, "container1", "container2"))...)
	// 3. Ping by hostname
	dockerCmd(t, append(runArgs, fmt.Sprintf(pingCmd, "fred", "wilma"))...)

	logDone("links - ping linked container")
}

func TestLinksPingLinkedContainersAfterRename(t *testing.T) {
	defer deleteAllContainers()

	out, _, _ := dockerCmd(t, "run", "-d", "--name", "container1", "busybox", "sleep", "10")
	idA := stripTrailingCharacters(out)
	out, _, _ = dockerCmd(t, "run", "-d", "--name", "container2", "busybox", "sleep", "10")
	idB := stripTrailingCharacters(out)
	dockerCmd(t, "rename", "container1", "container_new")
	dockerCmd(t, "run", "--rm", "--link", "container_new:alias1", "--link", "container2:alias2", "busybox", "sh", "-c", "ping -c 1 alias1 -W 1 && ping -c 1 alias2 -W 1")
	dockerCmd(t, "kill", idA)
	dockerCmd(t, "kill", idB)

	logDone("links - ping linked container after rename")
}

func TestLinksIpTablesRulesWhenLinkAndUnlink(t *testing.T) {
	testRequires(t, SameHostDaemon)
	defer deleteAllContainers()

	dockerCmd(t, "run", "-d", "--name", "child", "--publish", "8080:80", "busybox", "sleep", "10")
	dockerCmd(t, "run", "-d", "--name", "parent", "--link", "child:http", "busybox", "sleep", "10")

	childIP := findContainerIP(t, "child")
	parentIP := findContainerIP(t, "parent")

	sourceRule := []string{"-i", "docker0", "-o", "docker0", "-p", "tcp", "-s", childIP, "--sport", "80", "-d", parentIP, "-j", "ACCEPT"}
	destinationRule := []string{"-i", "docker0", "-o", "docker0", "-p", "tcp", "-s", parentIP, "--dport", "80", "-d", childIP, "-j", "ACCEPT"}
	if !iptables.Exists("filter", "DOCKER", sourceRule...) || !iptables.Exists("filter", "DOCKER", destinationRule...) {
		t.Fatal("Iptables rules not found")
	}

	dockerCmd(t, "rm", "--link", "parent/http")
	if iptables.Exists("filter", "DOCKER", sourceRule...) || iptables.Exists("filter", "DOCKER", destinationRule...) {
		t.Fatal("Iptables rules should be removed when unlink")
	}

	dockerCmd(t, "kill", "child")
	dockerCmd(t, "kill", "parent")

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
	logDone("link - container start successfully updating stopped parent links")
}

func TestLinksHostsFilesInject(t *testing.T) {
	testRequires(t, SameHostDaemon, ExecSupport)

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

	contentOne, err := readContainerFileWithExec(idOne, "/etc/hosts")
	if err != nil {
		t.Fatal(err, string(contentOne))
	}

	contentTwo, err := readContainerFileWithExec(idTwo, "/etc/hosts")
	if err != nil {
		t.Fatal(err, string(contentTwo))
	}

	if !strings.Contains(string(contentTwo), "onetwo") {
		t.Fatal("Host is not present in updated hosts file", string(contentTwo))
	}

	logDone("link - ensure containers hosts files are updated with the link alias.")
}

func TestLinksNetworkHostContainer(t *testing.T) {
	defer deleteAllContainers()

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "--net", "host", "--name", "host_container", "busybox", "top"))
	if err != nil {
		t.Fatal(err, out)
	}

	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "run", "--name", "should_fail", "--link", "host_container:tester", "busybox", "true"))
	if err == nil || !strings.Contains(out, "--net=host can't be used with links. This would result in undefined behavior.") {
		t.Fatalf("Running container linking to a container with --net host should have failed: %s", out)
	}

	logDone("link - error thrown when linking to container with --net host")
}

func TestLinksUpdateOnRestart(t *testing.T) {
	testRequires(t, SameHostDaemon, ExecSupport)

	defer deleteAllContainers()

	if out, err := exec.Command(dockerBinary, "run", "-d", "--name", "one", "busybox", "top").CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	out, err := exec.Command(dockerBinary, "run", "-d", "--name", "two", "--link", "one:onetwo", "--link", "one:one", "busybox", "top").CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	id := strings.TrimSpace(string(out))

	realIP, err := inspectField("one", "NetworkSettings.IPAddress")
	if err != nil {
		t.Fatal(err)
	}
	content, err := readContainerFileWithExec(id, "/etc/hosts")
	if err != nil {
		t.Fatal(err, string(content))
	}
	getIP := func(hosts []byte, hostname string) string {
		re := regexp.MustCompile(fmt.Sprintf(`(\S*)\t%s`, regexp.QuoteMeta(hostname)))
		matches := re.FindSubmatch(hosts)
		if matches == nil {
			t.Fatalf("Hostname %s have no matches in hosts", hostname)
		}
		return string(matches[1])
	}
	if ip := getIP(content, "one"); ip != realIP {
		t.Fatalf("For 'one' alias expected IP: %s, got: %s", realIP, ip)
	}
	if ip := getIP(content, "onetwo"); ip != realIP {
		t.Fatalf("For 'onetwo' alias expected IP: %s, got: %s", realIP, ip)
	}
	if out, err := exec.Command(dockerBinary, "restart", "one").CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	realIP, err = inspectField("one", "NetworkSettings.IPAddress")
	if err != nil {
		t.Fatal(err)
	}
	content, err = readContainerFileWithExec(id, "/etc/hosts")
	if err != nil {
		t.Fatal(err, string(content))
	}
	if ip := getIP(content, "one"); ip != realIP {
		t.Fatalf("For 'one' alias expected IP: %s, got: %s", realIP, ip)
	}
	if ip := getIP(content, "onetwo"); ip != realIP {
		t.Fatalf("For 'onetwo' alias expected IP: %s, got: %s", realIP, ip)
	}
	logDone("link - ensure containers hosts files are updated on restart")
}
