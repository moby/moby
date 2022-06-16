package main

import (
	"net"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/icmd"
)

type DockerCLIProxySuite struct {
	ds *DockerSuite
}

func (s *DockerCLIProxySuite) TearDownTest(c *testing.T) {
	s.ds.TearDownTest(c)
}

func (s *DockerCLIProxySuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

func (s *DockerCLIProxySuite) TestCLIProxyDisableProxyUnixSock(c *testing.T) {
	testRequires(c, DaemonIsLinux, testEnv.IsLocalDaemon)

	icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "info"},
		Env:     appendBaseEnv(false, "HTTP_PROXY=http://127.0.0.1:9999"),
	}).Assert(c, icmd.Success)
}

// Can't use localhost here since go has a special case to not use proxy if connecting to localhost
// See https://golang.org/pkg/net/http/#ProxyFromEnvironment
func (s *DockerDaemonSuite) TestCLIProxyProxyTCPSock(c *testing.T) {
	// get the IP to use to connect since we can't use localhost
	addrs, err := net.InterfaceAddrs()
	assert.NilError(c, err)
	var ip string
	for _, addr := range addrs {
		sAddr := addr.String()
		if !strings.Contains(sAddr, "127.0.0.1") {
			addrArr := strings.Split(sAddr, "/")
			ip = addrArr[0]
			break
		}
	}

	assert.Assert(c, ip != "")

	s.d.Start(c, "-H", "tcp://"+ip+":2375")

	icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "info"},
		Env:     []string{"DOCKER_HOST=tcp://" + ip + ":2375", "HTTP_PROXY=127.0.0.1:9999"},
	}).Assert(c, icmd.Expected{Error: "exit status 1", ExitCode: 1})
	// Test with no_proxy
	icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "info"},
		Env:     []string{"DOCKER_HOST=tcp://" + ip + ":2375", "HTTP_PROXY=127.0.0.1:9999", "NO_PROXY=" + ip},
	}).Assert(c, icmd.Success)
}
