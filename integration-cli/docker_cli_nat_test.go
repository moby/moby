package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"strings"

	"github.com/go-check/check"
)

func startServerContainer(c *check.C, msg string, port int) string {
	name := "server"
	cmd := []string{
		"-d",
		"-p", fmt.Sprintf("%d:%d", port, port),
		"busybox",
		"sh", "-c", fmt.Sprintf("echo %q | nc -lp %d", msg, port),
	}
	c.Assert(waitForContainer(name, cmd...), check.IsNil)
	return name
}

func getExternalAddress(c *check.C) net.IP {
	iface, err := net.InterfaceByName("eth0")
	if err != nil {
		c.Skip(fmt.Sprintf("Test not running with `make test`. Interface eth0 not found: %v", err))
	}

	ifaceAddrs, err := iface.Addrs()
	c.Assert(err, check.IsNil)
	c.Assert(len(ifaceAddrs), check.Not(check.Equals), 0)

	ifaceIP, _, err := net.ParseCIDR(ifaceAddrs[0].String())
	c.Assert(err, check.IsNil)

	return ifaceIP
}

func getContainerLogs(c *check.C, containerID string) string {
	out, _ := dockerCmd(c, "logs", containerID)
	return strings.Trim(out, "\r\n")
}

func getContainerStatus(c *check.C, containerID string) string {
	out, err := inspectField(containerID, "State.Running")
	c.Assert(err, check.IsNil)
	return out
}

func (s *DockerSuite) TestNetworkNat(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, SameHostDaemon, NativeExecDriver)
	msg := "it works"
	startServerContainer(c, msg, 8080)
	endpoint := getExternalAddress(c)
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", endpoint.String(), 8080))
	c.Assert(err, check.IsNil)

	data, err := ioutil.ReadAll(conn)
	conn.Close()
	c.Assert(err, check.IsNil)

	final := strings.TrimRight(string(data), "\n")
	c.Assert(final, check.Equals, msg, check.Commentf("Expected message %q but received %q", msg, final))
}

func (s *DockerSuite) TestNetworkLocalhostTCPNat(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, SameHostDaemon, NativeExecDriver)
	var (
		msg = "hi yall"
	)
	startServerContainer(c, msg, 8081)
	conn, err := net.Dial("tcp", "localhost:8081")
	c.Assert(err, check.IsNil)

	data, err := ioutil.ReadAll(conn)
	conn.Close()
	c.Assert(err, check.IsNil)

	final := strings.TrimRight(string(data), "\n")
	c.Assert(final, check.Equals, msg, check.Commentf("Expected message %q but received %q", msg, final))
}

func (s *DockerSuite) TestNetworkLoopbackNat(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, SameHostDaemon, NativeExecDriver, NotUserNamespace)
	msg := "it works"
	startServerContainer(c, msg, 8080)
	endpoint := getExternalAddress(c)
	out, _ := dockerCmd(c, "run", "-t", "--net=container:server", "busybox",
		"sh", "-c", fmt.Sprintf("stty raw && nc -w 5 %s 8080", endpoint.String()))
	final := strings.TrimRight(string(out), "\n")
	c.Assert(final, check.Equals, msg, check.Commentf("Expected message %q but received %q", msg, final))
}
