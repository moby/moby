package main

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/testutil"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type DockerCLIPortSuite struct {
	ds *DockerSuite
}

func (s *DockerCLIPortSuite) TearDownTest(ctx context.Context, c *testing.T) {
	s.ds.TearDownTest(ctx, c)
}

func (s *DockerCLIPortSuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

func (s *DockerCLIPortSuite) TestPortList(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	ctx := testutil.GetContext(c)

	// one port
	firstID := cli.DockerCmd(c, "run", "-d", "-p", "9876:80", "busybox", "top").Stdout()
	firstID = strings.TrimSpace(firstID)

	out := cli.DockerCmd(c, "port", firstID, "80").Stdout()

	assertPortList(c, out, []string{"0.0.0.0:9876", "[::]:9876"})

	out = cli.DockerCmd(c, "port", firstID).Stdout()

	assertPortList(c, out, []string{"80/tcp -> 0.0.0.0:9876", "80/tcp -> [::]:9876"})

	cli.DockerCmd(c, "rm", "-f", firstID)

	// three port
	id := cli.DockerCmd(c, "run", "-d",
		"-p", "9876:80",
		"-p", "9877:81",
		"-p", "9878:82",
		"busybox", "top",
	).Stdout()
	id = strings.TrimSpace(id)

	out = cli.DockerCmd(c, "port", id, "80").Stdout()

	assertPortList(c, out, []string{"0.0.0.0:9876", "[::]:9876"})

	out = cli.DockerCmd(c, "port", id).Stdout()

	assertPortList(c, out, []string{
		"80/tcp -> 0.0.0.0:9876",
		"80/tcp -> [::]:9876",
		"81/tcp -> 0.0.0.0:9877",
		"81/tcp -> [::]:9877",
		"82/tcp -> 0.0.0.0:9878",
		"82/tcp -> [::]:9878",
	})

	cli.DockerCmd(c, "rm", "-f", id)

	// more and one port mapped to the same container port
	id = cli.DockerCmd(c, "run", "-d",
		"-p", "9876:80",
		"-p", "9999:80",
		"-p", "9877:81",
		"-p", "9878:82",
		"busybox", "top",
	).Stdout()
	id = strings.TrimSpace(id)

	out = cli.DockerCmd(c, "port", id, "80").Stdout()

	assertPortList(c, out, []string{"0.0.0.0:9876", "[::]:9876", "0.0.0.0:9999", "[::]:9999"})

	out = cli.DockerCmd(c, "port", id).Stdout()

	assertPortList(c, out, []string{
		"80/tcp -> 0.0.0.0:9876",
		"80/tcp -> 0.0.0.0:9999",
		"80/tcp -> [::]:9876",
		"80/tcp -> [::]:9999",
		"81/tcp -> 0.0.0.0:9877",
		"81/tcp -> [::]:9877",
		"82/tcp -> 0.0.0.0:9878",
		"82/tcp -> [::]:9878",
	})
	cli.DockerCmd(c, "rm", "-f", id)

	testRange := func() {
		// host port ranges used
		IDs := make([]string, 3)
		for i := 0; i < 3; i++ {
			out = cli.DockerCmd(c, "run", "-d", "-p", "9090-9092:80", "busybox", "top").Stdout()
			IDs[i] = strings.TrimSpace(out)

			out = cli.DockerCmd(c, "port", IDs[i]).Stdout()

			assertPortList(c, out, []string{
				fmt.Sprintf("80/tcp -> 0.0.0.0:%d", 9090+i),
				fmt.Sprintf("80/tcp -> [::]:%d", 9090+i),
			})
		}

		// test port range exhaustion
		out, _, err := dockerCmdWithError("run", "-d", "-p", "9090-9092:80", "busybox", "top")
		// Exhausted port range did not return an error
		assert.Assert(c, err != nil, "out: %s", out)

		for i := 0; i < 3; i++ {
			cli.DockerCmd(c, "rm", "-f", IDs[i])
		}
	}
	testRange()
	// Verify we ran re-use port ranges after they are no longer in use.
	testRange()

	// test invalid port ranges
	for _, invalidRange := range []string{"9090-9089:80", "9090-:80", "-9090:80"} {
		out, _, err := dockerCmdWithError("run", "-d", "-p", invalidRange, "busybox", "top")
		// Port range should have returned an error
		assert.Assert(c, err != nil, "out: %s", out)
	}

	// test host range:container range spec.
	id = cli.DockerCmd(c, "run", "-d", "-p", "9800-9803:80-83", "busybox", "top").Stdout()
	id = strings.TrimSpace(id)

	out = cli.DockerCmd(c, "port", id).Stdout()

	assertPortList(c, out, []string{
		"80/tcp -> 0.0.0.0:9800",
		"80/tcp -> [::]:9800",
		"81/tcp -> 0.0.0.0:9801",
		"81/tcp -> [::]:9801",
		"82/tcp -> 0.0.0.0:9802",
		"82/tcp -> [::]:9802",
		"83/tcp -> 0.0.0.0:9803",
		"83/tcp -> [::]:9803",
	})
	cli.DockerCmd(c, "rm", "-f", id)

	// test mixing protocols in same port range
	id = cli.DockerCmd(c, "run", "-d", "-p", "8000-8080:80", "-p", "8000-8080:80/udp", "busybox", "top").Stdout()
	id = strings.TrimSpace(id)

	out = cli.DockerCmd(c, "port", id).Stdout()
	// Running this test multiple times causes the TCP port to increment.
	err := assertPortRange(ctx, id, []int{8000, 8080}, []int{8000, 8080})
	assert.Check(c, err)
	cli.DockerCmd(c, "rm", "-f", id)
}

func assertPortList(c *testing.T, out string, expected []string) {
	c.Helper()
	lines := strings.Split(strings.Trim(out, "\n "), "\n")
	assert.Assert(c, is.Len(lines, len(expected)), "expected: %s", strings.Join(expected, ", "))

	sort.Strings(lines)
	sort.Strings(expected)

	// "docker port" does not yet have a "--format" flag, and older versions
	// of the CLI used an incorrect output format for mappings on IPv6 addresses
	// for example, "80/tcp -> :::80" instead of "80/tcp -> [::]:80".
	oldFormat := func(mapping string) string {
		old := strings.Replace(mapping, "[", "", 1)
		old = strings.Replace(old, "]:", ":", 1)
		return old
	}

	for i := 0; i < len(expected); i++ {
		if lines[i] == expected[i] {
			continue
		}
		assert.Equal(c, lines[i], oldFormat(expected[i]))
	}
}

func assertPortRange(ctx context.Context, id string, expectedTCP, expectedUDP []int) error {
	client := testEnv.APIClient()
	inspect, err := client.ContainerInspect(ctx, id)
	if err != nil {
		return err
	}

	var validTCP, validUDP bool
	for portAndProto, binding := range inspect.NetworkSettings.Ports {
		if portAndProto.Proto() == "tcp" && len(expectedTCP) == 0 {
			continue
		}
		if portAndProto.Proto() == "udp" && len(expectedTCP) == 0 {
			continue
		}

		for _, b := range binding {
			port, err := strconv.Atoi(b.HostPort)
			if err != nil {
				return err
			}

			if len(expectedTCP) > 0 {
				if port < expectedTCP[0] || port > expectedTCP[1] {
					return fmt.Errorf("tcp port (%d) not in range expected range %d-%d", port, expectedTCP[0], expectedTCP[1])
				}
				validTCP = true
			}
			if len(expectedUDP) > 0 {
				if port < expectedUDP[0] || port > expectedUDP[1] {
					return fmt.Errorf("udp port (%d) not in range expected range %d-%d", port, expectedUDP[0], expectedUDP[1])
				}
				validUDP = true
			}
		}
	}
	if !validTCP {
		return fmt.Errorf("tcp port not found")
	}
	if !validUDP {
		return fmt.Errorf("udp port not found")
	}
	return nil
}

func stopRemoveContainer(id string, c *testing.T) {
	cli.DockerCmd(c, "rm", "-f", id)
}

func (s *DockerCLIPortSuite) TestUnpublishedPortsInPsOutput(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	// Run busybox with command line expose (equivalent to EXPOSE in image's Dockerfile) for the following ports
	port1 := 80
	port2 := 443
	expose1 := fmt.Sprintf("--expose=%d", port1)
	expose2 := fmt.Sprintf("--expose=%d", port2)
	cli.DockerCmd(c, "run", "-d", expose1, expose2, "busybox", "sleep", "5")

	// Check docker ps o/p for last created container reports the unpublished ports
	unpPort1 := fmt.Sprintf("%d/tcp", port1)
	unpPort2 := fmt.Sprintf("%d/tcp", port2)
	out := cli.DockerCmd(c, "ps", "-n=1").Stdout()
	// Missing unpublished ports in docker ps output
	assert.Assert(c, is.Contains(out, unpPort1))
	// Missing unpublished ports in docker ps output
	assert.Assert(c, is.Contains(out, unpPort2))
	// Run the container forcing to publish the exposed ports
	cli.DockerCmd(c, "run", "-d", "-P", expose1, expose2, "busybox", "sleep", "5")

	// Check docker ps o/p for last created container reports the exposed ports in the port bindings
	expBndRegx1 := regexp.MustCompile(`0.0.0.0:\d\d\d\d\d->` + unpPort1)
	expBndRegx2 := regexp.MustCompile(`0.0.0.0:\d\d\d\d\d->` + unpPort2)
	out = cli.DockerCmd(c, "ps", "-n=1").Stdout()
	// Cannot find expected port binding port (0.0.0.0:xxxxx->unpPort1) in docker ps output
	assert.Equal(c, expBndRegx1.MatchString(out), true, fmt.Sprintf("out: %s; unpPort1: %s", out, unpPort1))
	// Cannot find expected port binding port (0.0.0.0:xxxxx->unpPort2) in docker ps output
	assert.Equal(c, expBndRegx2.MatchString(out), true, fmt.Sprintf("out: %s; unpPort2: %s", out, unpPort2))

	// Run the container specifying explicit port bindings for the exposed ports
	offset := 10000
	pFlag1 := fmt.Sprintf("%d:%d", offset+port1, port1)
	pFlag2 := fmt.Sprintf("%d:%d", offset+port2, port2)

	id := cli.DockerCmd(c, "run", "-d", "-p", pFlag1, "-p", pFlag2, expose1, expose2, "busybox", "sleep", "5").Stdout()
	id = strings.TrimSpace(id)

	// Check docker ps o/p for last created container reports the specified port mappings
	expBnd1 := fmt.Sprintf("0.0.0.0:%d->%s", offset+port1, unpPort1)
	expBnd2 := fmt.Sprintf("0.0.0.0:%d->%s", offset+port2, unpPort2)
	out = cli.DockerCmd(c, "ps", "-n=1").Stdout()
	// Cannot find expected port binding (expBnd1) in docker ps output
	assert.Assert(c, is.Contains(out, expBnd1))
	// Cannot find expected port binding (expBnd2) in docker ps output
	assert.Assert(c, is.Contains(out, expBnd2))
	// Remove container now otherwise it will interfere with next test
	stopRemoveContainer(id, c)

	// Run the container with explicit port bindings and no exposed ports
	id = cli.DockerCmd(c, "run", "-d", "-p", pFlag1, "-p", pFlag2, "busybox", "sleep", "5").Stdout()
	id = strings.TrimSpace(id)

	// Check docker ps o/p for last created container reports the specified port mappings
	out = cli.DockerCmd(c, "ps", "-n=1").Stdout()
	// Cannot find expected port binding (expBnd1) in docker ps output
	assert.Assert(c, is.Contains(out, expBnd1))
	// Cannot find expected port binding (expBnd2) in docker ps output
	assert.Assert(c, is.Contains(out, expBnd2))
	// Remove container now otherwise it will interfere with next test
	stopRemoveContainer(id, c)

	// Run the container with one unpublished exposed port and one explicit port binding
	cli.DockerCmd(c, "run", "-d", expose1, "-p", pFlag2, "busybox", "sleep", "5")

	// Check docker ps o/p for last created container reports the specified unpublished port and port mapping
	out = cli.DockerCmd(c, "ps", "-n=1").Stdout()
	// Missing unpublished exposed ports (unpPort1) in docker ps output
	assert.Assert(c, is.Contains(out, unpPort1))
	// Missing port binding (expBnd2) in docker ps output
	assert.Assert(c, is.Contains(out, expBnd2))
}

func (s *DockerCLIPortSuite) TestPortHostBinding(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	firstID := cli.DockerCmd(c, "run", "-d", "-p", "9876:80", "busybox", "nc", "-l", "-p", "80").Stdout()
	firstID = strings.TrimSpace(firstID)

	out := cli.DockerCmd(c, "port", firstID, "80").Stdout()

	assertPortList(c, out, []string{"0.0.0.0:9876", "[::]:9876"})

	cli.DockerCmd(c, "run", "--net=host", "busybox", "nc", "localhost", "9876")

	cli.DockerCmd(c, "rm", "-f", firstID)

	out, _, err := dockerCmdWithError("run", "--net=host", "busybox", "nc", "localhost", "9876")
	// Port is still bound after the Container is removed
	assert.Assert(c, err != nil, out)
}

func (s *DockerCLIPortSuite) TestPortExposeHostBinding(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	firstID := cli.DockerCmd(c, "run", "-d", "-P", "--expose", "80", "busybox", "nc", "-l", "-p", "80").Stdout()
	firstID = strings.TrimSpace(firstID)

	exposedPort := cli.DockerCmd(c, "inspect", "--format", `{{index .NetworkSettings.Ports "80/tcp" 0 "HostPort" }}`, firstID).Stdout()
	exposedPort = strings.TrimSpace(exposedPort)
	cli.DockerCmd(c, "run", "--net=host", "busybox", "nc", "127.0.0.1", exposedPort)

	cli.DockerCmd(c, "rm", "-f", firstID)

	out, _, err := dockerCmdWithError("run", "--net=host", "busybox", "nc", "127.0.0.1", exposedPort)
	// Port is still bound after the Container is removed
	assert.Assert(c, err != nil, "out: %s", out)
}

func (s *DockerCLIPortSuite) TestPortBindingOnSandbox(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	cli.DockerCmd(c, "network", "create", "--internal", "-d", "bridge", "internal-net")
	nr := getNetworkResource(c, "internal-net")
	assert.Equal(c, nr.Internal, true)

	cli.DockerCmd(c, "run", "--net", "internal-net", "-d", "--name", "c1", "-p", "8080:8080", "busybox", "nc", "-l", "-p", "8080")
	cli.WaitRun(c, "c1")

	_, _, err := dockerCmdWithError("run", "--net=host", "busybox", "nc", "localhost", "8080")
	assert.Assert(c, err != nil, "Port mapping on internal network is expected to fail")
	// Connect container to another normal bridge network
	cli.DockerCmd(c, "network", "create", "-d", "bridge", "foo-net")
	cli.DockerCmd(c, "network", "connect", "foo-net", "c1")

	_, _, err = dockerCmdWithError("run", "--net=host", "busybox", "nc", "localhost", "8080")
	assert.Assert(c, err == nil, "Port mapping on the new network is expected to succeed")
}
