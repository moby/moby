package main

import (
	"github.com/go-check/check"
	"gotest.tools/icmd"
)

func (s *DockerSuite) TestCLIProxyDisableProxyUnixSock(c *check.C) {
	testRequires(c, DaemonIsLinux, testEnv.IsLocalDaemon)

	icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "info"},
		Env:     appendBaseEnv(false, "HTTP_PROXY=http://127.0.0.1:9999"),
	}).Assert(c, icmd.Success)
}

// Can't use localhost here since go has a special case to not use proxy if connecting to localhost
// See https://golang.org/pkg/net/http/#ProxyFromEnvironment
func (s *DockerDaemonSuite) TestCLIProxyProxyTCPSock(c *check.C) {
	ip := s.d.IP(c).String()
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
