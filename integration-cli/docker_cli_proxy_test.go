package main

import (
	"os/exec"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestCliProxyDisableProxyUnixSock(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, SameHostDaemon) // test is valid when DOCKER_HOST=unix://..

	cmd := exec.Command(dockerBinary, "info")
	cmd.Env = appendBaseEnv(false, "HTTP_PROXY=http://127.0.0.1:9999")

	out, _, err := runCommandWithOutput(cmd)
	c.Assert(err, checker.IsNil, check.Commentf("%v", out))

}

// Can't use localhost here since go has a special case to not use proxy if connecting to localhost
// See https://golang.org/pkg/net/http/#ProxyFromEnvironment
func (s *DockerDaemonSuite) TestCliProxyProxyTCPSock(c *check.C) {
	testRequires(c, SameHostDaemon)

	c.Assert(s.d.Start(), checker.IsNil)
	c.Assert(s.d.ip, checker.Not(checker.Equals), "")

	cmd := exec.Command(dockerBinary, "info")
	cmd.Env = []string{"DOCKER_HOST=tcp://" + s.d.ip + ":2375", "HTTP_PROXY=127.0.0.1:9999"}
	out, _, err := runCommandWithOutput(cmd)
	c.Assert(err, checker.NotNil, check.Commentf("%v", out))
	// Test with no_proxy
	cmd.Env = append(cmd.Env, "NO_PROXY="+s.d.ip)
	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "info"))
	c.Assert(err, checker.IsNil, check.Commentf("%v", out))
}
