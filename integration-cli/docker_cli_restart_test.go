package main

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestRestartStoppedContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "echo", "foobar")

	cleanedContainerID := strings.TrimSpace(out)
	dockerCmd(c, "wait", cleanedContainerID)

	out, _ = dockerCmd(c, "logs", cleanedContainerID)
	c.Assert(out, checker.Equals, "foobar\n")

	dockerCmd(c, "restart", cleanedContainerID)

	out, _ = dockerCmd(c, "logs", cleanedContainerID)
	c.Assert(out, checker.Equals, "foobar\nfoobar\n")
}

func (s *DockerSuite) TestRestartRunningContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "sh", "-c", "echo foobar && sleep 30 && echo 'should not print this'")

	cleanedContainerID := strings.TrimSpace(out)

	c.Assert(waitRun(cleanedContainerID), checker.IsNil)

	out, _ = dockerCmd(c, "logs", cleanedContainerID)
	c.Assert(out, checker.Equals, "foobar\n")

	dockerCmd(c, "restart", "-t", "1", cleanedContainerID)

	out, _ = dockerCmd(c, "logs", cleanedContainerID)

	c.Assert(waitRun(cleanedContainerID), checker.IsNil)

	c.Assert(out, checker.Equals, "foobar\nfoobar\n")
}

// Test that restarting a container with a volume does not create a new volume on restart. Regression test for #819.
func (s *DockerSuite) TestRestartWithVolumes(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "-v", "/test", "busybox", "top")

	cleanedContainerID := strings.TrimSpace(out)
	out, err := inspectFilter(cleanedContainerID, "len .Mounts")
	c.Assert(err, check.IsNil, check.Commentf("failed to inspect %s: %s", cleanedContainerID, out))
	out = strings.Trim(out, " \n\r")
	c.Assert(out, checker.Equals, "1")

	source, err := inspectMountSourceField(cleanedContainerID, "/test")
	c.Assert(err, checker.IsNil)

	dockerCmd(c, "restart", cleanedContainerID)

	out, err = inspectFilter(cleanedContainerID, "len .Mounts")
	c.Assert(err, check.IsNil, check.Commentf("failed to inspect %s: %s", cleanedContainerID, out))
	out = strings.Trim(out, " \n\r")
	c.Assert(out, checker.Equals, "1")

	sourceAfterRestart, err := inspectMountSourceField(cleanedContainerID, "/test")
	c.Assert(err, checker.IsNil)
	c.Assert(source, checker.Equals, sourceAfterRestart)
}

func (s *DockerSuite) TestRestartPolicyNO(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--restart=no", "busybox", "false")

	id := strings.TrimSpace(string(out))
	name := inspectField(c, id, "HostConfig.RestartPolicy.Name")
	c.Assert(name, checker.Equals, "no")
}

func (s *DockerSuite) TestRestartPolicyAlways(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--restart=always", "busybox", "false")

	id := strings.TrimSpace(string(out))
	name := inspectField(c, id, "HostConfig.RestartPolicy.Name")
	c.Assert(name, checker.Equals, "always")

	MaximumRetryCount := inspectField(c, id, "HostConfig.RestartPolicy.MaximumRetryCount")

	// MaximumRetryCount=0 if the restart policy is always
	c.Assert(MaximumRetryCount, checker.Equals, "0")
}

func (s *DockerSuite) TestRestartPolicyOnFailure(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--restart=on-failure:1", "busybox", "false")

	id := strings.TrimSpace(string(out))
	name := inspectField(c, id, "HostConfig.RestartPolicy.Name")
	c.Assert(name, checker.Equals, "on-failure")

}

// a good container with --restart=on-failure:3
// MaximumRetryCount!=0; RestartCount=0
func (s *DockerSuite) TestContainerRestartwithGoodContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--restart=on-failure:3", "busybox", "true")

	id := strings.TrimSpace(string(out))
	err := waitInspect(id, "{{ .State.Restarting }} {{ .State.Running }}", "false false", 5*time.Second)
	c.Assert(err, checker.IsNil)

	count := inspectField(c, id, "RestartCount")
	c.Assert(count, checker.Equals, "0")

	MaximumRetryCount := inspectField(c, id, "HostConfig.RestartPolicy.MaximumRetryCount")
	c.Assert(MaximumRetryCount, checker.Equals, "3")

}

func (s *DockerSuite) TestContainerRestartSuccess(c *check.C) {
	testRequires(c, DaemonIsLinux, SameHostDaemon)

	out, _ := dockerCmd(c, "run", "-d", "--restart=always", "busybox", "top")
	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), check.IsNil)

	pidStr := inspectField(c, id, "State.Pid")

	pid, err := strconv.Atoi(pidStr)
	c.Assert(err, check.IsNil)

	p, err := os.FindProcess(pid)
	c.Assert(err, check.IsNil)
	c.Assert(p, check.NotNil)

	err = p.Kill()
	c.Assert(err, check.IsNil)

	err = waitInspect(id, "{{.RestartCount}}", "1", 5*time.Second)
	c.Assert(err, check.IsNil)

	err = waitInspect(id, "{{.State.Status}}", "running", 5*time.Second)
	c.Assert(err, check.IsNil)
}

func (s *DockerSuite) TestUserDefinedNetworkWithRestartPolicy(c *check.C) {
	testRequires(c, DaemonIsLinux, SameHostDaemon, NotUserNamespace, NotArm)
	dockerCmd(c, "network", "create", "-d", "bridge", "udNet")

	dockerCmd(c, "run", "-d", "--net=udNet", "--name=first", "busybox", "top")
	c.Assert(waitRun("first"), check.IsNil)

	dockerCmd(c, "run", "-d", "--restart=always", "--net=udNet", "--name=second",
		"--link=first:foo", "busybox", "top")
	c.Assert(waitRun("second"), check.IsNil)

	// ping to first and its alias foo must succeed
	_, _, err := dockerCmdWithError("exec", "second", "ping", "-c", "1", "first")
	c.Assert(err, check.IsNil)
	_, _, err = dockerCmdWithError("exec", "second", "ping", "-c", "1", "foo")
	c.Assert(err, check.IsNil)

	// Now kill the second container and let the restart policy kick in
	pidStr := inspectField(c, "second", "State.Pid")

	pid, err := strconv.Atoi(pidStr)
	c.Assert(err, check.IsNil)

	p, err := os.FindProcess(pid)
	c.Assert(err, check.IsNil)
	c.Assert(p, check.NotNil)

	err = p.Kill()
	c.Assert(err, check.IsNil)

	err = waitInspect("second", "{{.RestartCount}}", "1", 5*time.Second)
	c.Assert(err, check.IsNil)

	err = waitInspect("second", "{{.State.Status}}", "running", 5*time.Second)

	// ping to first and its alias foo must still succeed
	_, _, err = dockerCmdWithError("exec", "second", "ping", "-c", "1", "first")
	c.Assert(err, check.IsNil)
	_, _, err = dockerCmdWithError("exec", "second", "ping", "-c", "1", "foo")
	c.Assert(err, check.IsNil)
}
