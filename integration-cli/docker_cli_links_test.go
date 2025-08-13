package main

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/moby/moby/v2/integration-cli/cli"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type DockerCLILinksSuite struct {
	ds *DockerSuite
}

func (s *DockerCLILinksSuite) TearDownTest(ctx context.Context, t *testing.T) {
	s.ds.TearDownTest(ctx, t)
}

func (s *DockerCLILinksSuite) OnTimeout(t *testing.T) {
	s.ds.OnTimeout(t)
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
	assert.Check(c, is.ErrorContains(err, "could not get container for bogus"))
	assert.Check(c, is.Contains(out, "could not get container"))
}

func (s *DockerCLILinksSuite) TestLinksPingLinkedContainers(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	// Test with the three different ways of specifying the default network on Linux
	testLinkPingOnNetwork(c, "")
	testLinkPingOnNetwork(c, "default")
	testLinkPingOnNetwork(c, "bridge")
}

func testLinkPingOnNetwork(t *testing.T, network string) {
	var postArgs []string
	if network != "" {
		postArgs = append(postArgs, []string{"--net", network}...)
	}
	postArgs = append(postArgs, []string{"busybox", "top"}...)
	runArgs1 := append([]string{"run", "-d", "--name", "container1", "--hostname", "fred"}, postArgs...)
	runArgs2 := append([]string{"run", "-d", "--name", "container2", "--hostname", "wilma"}, postArgs...)

	// Run the two named containers
	cli.DockerCmd(t, runArgs1...)
	cli.DockerCmd(t, runArgs2...)

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
	cli.DockerCmd(t, append(runArgs, fmt.Sprintf(pingCmd, "alias1", "alias2"))...)
	// 2. Ping by container name
	cli.DockerCmd(t, append(runArgs, fmt.Sprintf(pingCmd, "container1", "container2"))...)
	// 3. Ping by hostname
	cli.DockerCmd(t, append(runArgs, fmt.Sprintf(pingCmd, "fred", "wilma"))...)

	// Clean for next round
	cli.DockerCmd(t, "rm", "-f", "container1")
	cli.DockerCmd(t, "rm", "-f", "container2")
}

func (s *DockerCLILinksSuite) TestLinksPingLinkedContainersAfterRename(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	idA := cli.DockerCmd(c, "run", "-d", "--name", "container1", "busybox", "top").Stdout()
	idB := cli.DockerCmd(c, "run", "-d", "--name", "container2", "busybox", "top").Stdout()
	cli.DockerCmd(c, "rename", "container1", "container_new")
	cli.DockerCmd(c, "run", "--rm", "--link", "container_new:alias1", "--link", "container2:alias2", "busybox", "sh", "-c", "ping -c 1 alias1 -W 1 && ping -c 1 alias2 -W 1")
	cli.DockerCmd(c, "kill", strings.TrimSpace(idA))
	cli.DockerCmd(c, "kill", strings.TrimSpace(idB))
}

func (s *DockerCLILinksSuite) TestLinksInspectLinksStarted(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	cli.DockerCmd(c, "run", "-d", "--name", "container1", "busybox", "top")
	cli.DockerCmd(c, "run", "-d", "--name", "container2", "busybox", "top")
	cli.DockerCmd(c, "run", "-d", "--name", "testinspectlink", "--link", "container1:alias1", "--link", "container2:alias2", "busybox", "top")
	links := inspectFieldJSON(c, "testinspectlink", "HostConfig.Links")

	var result []string
	err := json.Unmarshal([]byte(links), &result)
	assert.NilError(c, err)

	expected := []string{
		"/container1:/testinspectlink/alias1",
		"/container2:/testinspectlink/alias2",
	}
	sort.Strings(result)
	assert.DeepEqual(c, result, expected)
}

func (s *DockerCLILinksSuite) TestLinksInspectLinksStopped(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	cli.DockerCmd(c, "run", "-d", "--name", "container1", "busybox", "top")
	cli.DockerCmd(c, "run", "-d", "--name", "container2", "busybox", "top")
	cli.DockerCmd(c, "run", "-d", "--name", "testinspectlink", "--link", "container1:alias1", "--link", "container2:alias2", "busybox", "true")
	links := inspectFieldJSON(c, "testinspectlink", "HostConfig.Links")

	var result []string
	err := json.Unmarshal([]byte(links), &result)
	assert.NilError(c, err)

	expected := []string{
		"/container1:/testinspectlink/alias1",
		"/container2:/testinspectlink/alias2",
	}
	sort.Strings(result)
	assert.DeepEqual(c, result, expected)
}

func (s *DockerCLILinksSuite) TestLinksNotStartedParentNotFail(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	cli.DockerCmd(c, "create", "--name=first", "busybox", "top")
	cli.DockerCmd(c, "create", "--name=second", "--link=first:first", "busybox", "top")
	cli.DockerCmd(c, "start", "first")
}

func (s *DockerCLILinksSuite) TestLinksHostsFilesInject(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, testEnv.IsLocalDaemon)

	idOne := cli.DockerCmd(c, "run", "-itd", "--name", "one", "busybox", "top").Stdout()
	idOne = strings.TrimSpace(idOne)
	idTwo := cli.DockerCmd(c, "run", "-itd", "--name", "two", "--link", "one:onetwo", "busybox", "top").Stdout()
	idTwo = strings.TrimSpace(idTwo)
	cli.WaitRun(c, idTwo)

	readContainerFileWithExec(c, idOne, "/etc/hosts")
	contentTwo := readContainerFileWithExec(c, idTwo, "/etc/hosts")
	// Host is not present in updated hosts file
	assert.Assert(c, is.Contains(string(contentTwo), "onetwo"))
}

func (s *DockerCLILinksSuite) TestLinksUpdateOnRestart(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, testEnv.IsLocalDaemon)
	cli.DockerCmd(c, "run", "-d", "--name", "one", "busybox", "top")
	id := cli.DockerCmd(c, "run", "-d", "--name", "two", "--link", "one:onetwo", "--link", "one:one", "busybox", "top").Stdout()
	id = strings.TrimSpace(id)

	realIP := inspectField(c, "one", "NetworkSettings.Networks.bridge.IPAddress")
	content := readContainerFileWithExec(c, id, "/etc/hosts")

	getIP := func(hosts []byte, hostname string) string {
		re := regexp.MustCompile(fmt.Sprintf(`(\S*)\t%s`, regexp.QuoteMeta(hostname)))
		matches := re.FindSubmatch(hosts)
		assert.Assert(c, matches != nil, "Hostname %s have no matches in hosts", hostname)
		return string(matches[1])
	}
	ip := getIP(content, "one")
	assert.Check(c, is.Equal(ip, realIP))

	ip = getIP(content, "onetwo")
	assert.Check(c, is.Equal(ip, realIP))

	cli.DockerCmd(c, "restart", "one")
	realIP = inspectField(c, "one", "NetworkSettings.Networks.bridge.IPAddress")

	content = readContainerFileWithExec(c, id, "/etc/hosts")
	ip = getIP(content, "one")
	assert.Check(c, is.Equal(ip, realIP))

	ip = getIP(content, "onetwo")
	assert.Check(c, is.Equal(ip, realIP))
}

func (s *DockerCLILinksSuite) TestLinkShortDefinition(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	cid := cli.DockerCmd(c, "run", "-d", "--name", "shortlinkdef", "busybox", "top").Stdout()
	cid = strings.TrimSpace(cid)
	cli.WaitRun(c, cid)

	cid2 := cli.DockerCmd(c, "run", "-d", "--name", "link2", "--link", "shortlinkdef", "busybox", "top").Stdout()
	cid2 = strings.TrimSpace(cid2)
	cli.WaitRun(c, cid2)

	links := inspectFieldJSON(c, cid2, "HostConfig.Links")
	assert.Equal(c, links, `["/shortlinkdef:/link2/shortlinkdef"]`)
}

func (s *DockerCLILinksSuite) TestLinksNetworkHostContainer(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	cli.DockerCmd(c, "run", "-d", "--net", "host", "--name", "host_container", "busybox", "top")
	out, _, err := dockerCmdWithError("run", "--name", "should_fail", "--link", "host_container:tester", "busybox", "true")

	// Running container linking to a container with --net host should have failed
	assert.Check(c, err != nil, "out: %s", out)
	// Running container linking to a container with --net host should have failed
	assert.Check(c, is.Contains(out, "conflicting options: host type networking can't be used with links. This would result in undefined behavior"))
}

func (s *DockerCLILinksSuite) TestLinksEtcHostsRegularFile(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	out := cli.DockerCmd(c, "run", "--net=host", "busybox", "ls", "-la", "/etc/hosts").Stdout()
	// /etc/hosts should be a regular file
	assert.Assert(c, is.Regexp("^-.+\n$", out))
}

func (s *DockerCLILinksSuite) TestLinksMultipleWithSameName(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	cli.DockerCmd(c, "run", "-d", "--name=upstream-a", "busybox", "top")
	cli.DockerCmd(c, "run", "-d", "--name=upstream-b", "busybox", "top")
	cli.DockerCmd(c, "run", "--link", "upstream-a:upstream", "--link", "upstream-b:upstream", "busybox", "sh", "-c", "ping -c 1 upstream")
}
