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
	out, _ = dockerCmd(c, "inspect", "--format", "{{ len .Mounts }}", cleanedContainerID)
	out = strings.Trim(out, " \n\r")
	c.Assert(out, checker.Equals, "1")

	source, err := inspectMountSourceField(cleanedContainerID, "/test")
	c.Assert(err, checker.IsNil)

	dockerCmd(c, "restart", cleanedContainerID)

	out, _ = dockerCmd(c, "inspect", "--format", "{{ len .Mounts }}", cleanedContainerID)
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
	name, err := inspectField(id, "HostConfig.RestartPolicy.Name")
	c.Assert(err, checker.IsNil)
	c.Assert(name, checker.Equals, "no")
}

func (s *DockerSuite) TestRestartPolicyAlways(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--restart=always", "busybox", "false")

	id := strings.TrimSpace(string(out))
	name, err := inspectField(id, "HostConfig.RestartPolicy.Name")
	c.Assert(err, checker.IsNil)
	c.Assert(name, checker.Equals, "always")

	MaximumRetryCount, err := inspectField(id, "HostConfig.RestartPolicy.MaximumRetryCount")
	c.Assert(err, checker.IsNil)

	// MaximumRetryCount=0 if the restart policy is always
	c.Assert(MaximumRetryCount, checker.Equals, "0")
}

func (s *DockerSuite) TestRestartPolicyOnFailure(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--restart=on-failure:1", "busybox", "false")

	id := strings.TrimSpace(string(out))
	name, err := inspectField(id, "HostConfig.RestartPolicy.Name")
	c.Assert(err, checker.IsNil)
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

	count, err := inspectField(id, "RestartCount")
	c.Assert(err, checker.IsNil)
	c.Assert(count, checker.Equals, "0")

	MaximumRetryCount, err := inspectField(id, "HostConfig.RestartPolicy.MaximumRetryCount")
	c.Assert(err, checker.IsNil)
	c.Assert(MaximumRetryCount, checker.Equals, "3")

}

func (s *DockerSuite) TestContainerRestartSuccess(c *check.C) {
	testRequires(c, DaemonIsLinux, SameHostDaemon)

	out, _ := dockerCmd(c, "run", "-d", "--restart=always", "busybox", "top")
	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), check.IsNil)

	pidStr, err := inspectField(id, "State.Pid")
	c.Assert(err, check.IsNil)

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
