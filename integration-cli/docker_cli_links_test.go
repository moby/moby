package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/docker/docker/runconfig"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

type DockerCLILinksSuite struct {
	ds *DockerSuite
}

func (s *DockerCLILinksSuite) TearDownTest(c *testing.T) {
	s.ds.TearDownTest(c)
}

func (s *DockerCLILinksSuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

func (s *DockerCLILinksSuite) TestLinksPingUnlinkedContainers(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	_, exitCode, err := dockerCmdWithError("run", "--rm", "busybox", "sh", "-c", "ping -c 1 alias1 -W 1 && ping -c 1 alias2 -W 1")

	// run ping failed with error
	assert.Equal(c, exitCode, 1, fmt.Sprintf("error: %v", err))
}

// Test for appropriate error when calling --link with an invalid target container
func (s *DockerCLILinksSuite) TestLinksInvalidContainerTarget(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	out, _, err := dockerCmdWithError("run", "--link", "bogus:alias", "busybox", "true")

	// an invalid container target should produce an error
	assert.Assert(c, err != nil, "out: %s", out)
	// an invalid container target should produce an error
	// note: convert the output to lowercase first as the error string
	// capitalization was changed after API version 1.32
	assert.Assert(c, strings.Contains(strings.ToLower(out), "could not get container"))
}

func (s *DockerCLILinksSuite) TestLinksPingLinkedContainers(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	// Test with the three different ways of specifying the default network on Linux
	testLinkPingOnNetwork(c, "")
	testLinkPingOnNetwork(c, "default")
	testLinkPingOnNetwork(c, "bridge")
}

func testLinkPingOnNetwork(c *testing.T, network string) {
	var postArgs []string
	if network != "" {
		postArgs = append(postArgs, []string{"--net", network}...)
	}
	postArgs = append(postArgs, []string{"busybox", "top"}...)
	runArgs1 := append([]string{"run", "-d", "--name", "container1", "--hostname", "fred"}, postArgs...)
	runArgs2 := append([]string{"run", "-d", "--name", "container2", "--hostname", "wilma"}, postArgs...)

	// Run the two named containers
	dockerCmd(c, runArgs1...)
	dockerCmd(c, runArgs2...)

	postArgs = []string{}
	if network != "" {
		postArgs = append(postArgs, []string{"--net", network}...)
	}
	postArgs = append(postArgs, []string{"busybox", "sh", "-c"}...)

	// Format a run for a container which links to the other two
	runArgs := append([]string{"run", "--rm", "--link", "container1:alias1", "--link", "container2:alias2"}, postArgs...)
	pingCmd := "ping -c 1 %s -W 1 && ping -c 1 %s -W 1"

	// test ping by alias, ping by name, and ping by hostname
	// 1. Ping by alias
	dockerCmd(c, append(runArgs, fmt.Sprintf(pingCmd, "alias1", "alias2"))...)
	// 2. Ping by container name
	dockerCmd(c, append(runArgs, fmt.Sprintf(pingCmd, "container1", "container2"))...)
	// 3. Ping by hostname
	dockerCmd(c, append(runArgs, fmt.Sprintf(pingCmd, "fred", "wilma"))...)

	// Clean for next round
	dockerCmd(c, "rm", "-f", "container1")
	dockerCmd(c, "rm", "-f", "container2")
}

func (s *DockerCLILinksSuite) TestLinksPingLinkedContainersAfterRename(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--name", "container1", "busybox", "top")
	idA := strings.TrimSpace(out)
	out, _ = dockerCmd(c, "run", "-d", "--name", "container2", "busybox", "top")
	idB := strings.TrimSpace(out)
	dockerCmd(c, "rename", "container1", "container_new")
	dockerCmd(c, "run", "--rm", "--link", "container_new:alias1", "--link", "container2:alias2", "busybox", "sh", "-c", "ping -c 1 alias1 -W 1 && ping -c 1 alias2 -W 1")
	dockerCmd(c, "kill", idA)
	dockerCmd(c, "kill", idB)
}

func (s *DockerCLILinksSuite) TestLinksInspectLinksStarted(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "--name", "container1", "busybox", "top")
	dockerCmd(c, "run", "-d", "--name", "container2", "busybox", "top")
	dockerCmd(c, "run", "-d", "--name", "testinspectlink", "--link", "container1:alias1", "--link", "container2:alias2", "busybox", "top")
	links := inspectFieldJSON(c, "testinspectlink", "HostConfig.Links")

	var result []string
	err := json.Unmarshal([]byte(links), &result)
	assert.NilError(c, err)

	var expected = []string{
		"/container1:/testinspectlink/alias1",
		"/container2:/testinspectlink/alias2",
	}
	sort.Strings(result)
	assert.DeepEqual(c, result, expected)
}

func (s *DockerCLILinksSuite) TestLinksInspectLinksStopped(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	dockerCmd(c, "run", "-d", "--name", "container1", "busybox", "top")
	dockerCmd(c, "run", "-d", "--name", "container2", "busybox", "top")
	dockerCmd(c, "run", "-d", "--name", "testinspectlink", "--link", "container1:alias1", "--link", "container2:alias2", "busybox", "true")
	links := inspectFieldJSON(c, "testinspectlink", "HostConfig.Links")

	var result []string
	err := json.Unmarshal([]byte(links), &result)
	assert.NilError(c, err)

	var expected = []string{
		"/container1:/testinspectlink/alias1",
		"/container2:/testinspectlink/alias2",
	}
	sort.Strings(result)
	assert.DeepEqual(c, result, expected)
}

func (s *DockerCLILinksSuite) TestLinksNotStartedParentNotFail(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "create", "--name=first", "busybox", "top")
	dockerCmd(c, "create", "--name=second", "--link=first:first", "busybox", "top")
	dockerCmd(c, "start", "first")
}

func (s *DockerCLILinksSuite) TestLinksHostsFilesInject(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, testEnv.IsLocalDaemon)

	out, _ := dockerCmd(c, "run", "-itd", "--name", "one", "busybox", "top")
	idOne := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "run", "-itd", "--name", "two", "--link", "one:onetwo", "busybox", "top")
	idTwo := strings.TrimSpace(out)

	assert.Assert(c, waitRun(idTwo) == nil)

	readContainerFileWithExec(c, idOne, "/etc/hosts")
	contentTwo := readContainerFileWithExec(c, idTwo, "/etc/hosts")
	// Host is not present in updated hosts file
	assert.Assert(c, strings.Contains(string(contentTwo), "onetwo"))
}

func (s *DockerCLILinksSuite) TestLinksUpdateOnRestart(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, testEnv.IsLocalDaemon)
	dockerCmd(c, "run", "-d", "--name", "one", "busybox", "top")
	out, _ := dockerCmd(c, "run", "-d", "--name", "two", "--link", "one:onetwo", "--link", "one:one", "busybox", "top")
	id := strings.TrimSpace(out)

	realIP := inspectField(c, "one", "NetworkSettings.Networks.bridge.IPAddress")
	content := readContainerFileWithExec(c, id, "/etc/hosts")

	getIP := func(hosts []byte, hostname string) string {
		re := regexp.MustCompile(fmt.Sprintf(`(\S*)\t%s`, regexp.QuoteMeta(hostname)))
		matches := re.FindSubmatch(hosts)
		assert.Assert(c, matches != nil, "Hostname %s have no matches in hosts", hostname)
		return string(matches[1])
	}
	ip := getIP(content, "one")
	assert.Equal(c, ip, realIP)

	ip = getIP(content, "onetwo")
	assert.Equal(c, ip, realIP)

	dockerCmd(c, "restart", "one")
	realIP = inspectField(c, "one", "NetworkSettings.Networks.bridge.IPAddress")

	content = readContainerFileWithExec(c, id, "/etc/hosts")
	ip = getIP(content, "one")
	assert.Equal(c, ip, realIP)

	ip = getIP(content, "onetwo")
	assert.Equal(c, ip, realIP)
}

func (s *DockerCLILinksSuite) TestLinksEnvs(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "-e", "e1=", "-e", "e2=v2", "-e", "e3=v3=v3", "--name=first", "busybox", "top")
	out, _ := dockerCmd(c, "run", "--name=second", "--link=first:first", "busybox", "env")
	assert.Assert(c, strings.Contains(out, "FIRST_ENV_e1=\n"))
	assert.Assert(c, strings.Contains(out, "FIRST_ENV_e2=v2"))
	assert.Assert(c, strings.Contains(out, "FIRST_ENV_e3=v3=v3"))
}

func (s *DockerCLILinksSuite) TestLinkShortDefinition(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--name", "shortlinkdef", "busybox", "top")

	cid := strings.TrimSpace(out)
	assert.Assert(c, waitRun(cid) == nil)

	out, _ = dockerCmd(c, "run", "-d", "--name", "link2", "--link", "shortlinkdef", "busybox", "top")

	cid2 := strings.TrimSpace(out)
	assert.Assert(c, waitRun(cid2) == nil)

	links := inspectFieldJSON(c, cid2, "HostConfig.Links")
	assert.Equal(c, links, "[\"/shortlinkdef:/link2/shortlinkdef\"]")
}

func (s *DockerCLILinksSuite) TestLinksNetworkHostContainer(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	dockerCmd(c, "run", "-d", "--net", "host", "--name", "host_container", "busybox", "top")
	out, _, err := dockerCmdWithError("run", "--name", "should_fail", "--link", "host_container:tester", "busybox", "true")

	// Running container linking to a container with --net host should have failed
	assert.Assert(c, err != nil, "out: %s", out)
	// Running container linking to a container with --net host should have failed
	assert.Assert(c, strings.Contains(out, runconfig.ErrConflictHostNetworkAndLinks.Error()))
}

func (s *DockerCLILinksSuite) TestLinksEtcHostsRegularFile(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	out, _ := dockerCmd(c, "run", "--net=host", "busybox", "ls", "-la", "/etc/hosts")
	// /etc/hosts should be a regular file
	assert.Assert(c, cmp.Regexp("^-.+\n$", out))
}

func (s *DockerCLILinksSuite) TestLinksMultipleWithSameName(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "--name=upstream-a", "busybox", "top")
	dockerCmd(c, "run", "-d", "--name=upstream-b", "busybox", "top")
	dockerCmd(c, "run", "--link", "upstream-a:upstream", "--link", "upstream-b:upstream", "busybox", "sh", "-c", "ping -c 1 upstream")
}
