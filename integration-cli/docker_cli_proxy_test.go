package main

import (
	"net"
	"os/exec"
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestCliProxyDisableProxyUnixSock(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, SameHostDaemon) // test is valid when DOCKER_HOST=unix://..

	cmd := exec.Command(dockerBinary, "info")
	cmd.Env = appendBaseEnv([]string{"HTTP_PROXY=http://127.0.0.1:9999"})

	out, _, err := runCommandWithOutput(cmd)
	c.Assert(err, checker.IsNil, check.Commentf("%v", out))

}

// Can't use localhost here since go has a special case to not use proxy if connecting to localhost
// See https://golang.org/pkg/net/http/#ProxyFromEnvironment
func (s *DockerDaemonSuite) TestCliProxyProxyTCPSock(c *check.C) {
	testRequires(c, SameHostDaemon)
	// get the IP to use to connect since we can't use localhost
	addrs, err := net.InterfaceAddrs()
	c.Assert(err, checker.IsNil)
	var ip string
	for _, addr := range addrs {
		sAddr := addr.String()
		if !strings.Contains(sAddr, "127.0.0.1") {
			addrArr := strings.Split(sAddr, "/")
			ip = addrArr[0]
			break
		}
	}

	c.Assert(ip, checker.Not(checker.Equals), "")

	err = s.d.Start("-H", "tcp://"+ip+":2375")
	c.Assert(err, checker.IsNil)
	cmd := exec.Command(dockerBinary, "info")
	cmd.Env = []string{"DOCKER_HOST=tcp://" + ip + ":2375", "HTTP_PROXY=127.0.0.1:9999"}
	out, _, err := runCommandWithOutput(cmd)
	c.Assert(err, checker.NotNil, check.Commentf("%v", out))
	// Test with no_proxy
	cmd.Env = append(cmd.Env, "NO_PROXY="+ip)
	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "info"))
	c.Assert(err, checker.IsNil, check.Commentf("%v", out))
}
