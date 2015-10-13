package main

import (
	"strings"
	"time"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestRestartStoppedContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "echo", "foobar")

	cleanedContainerID := strings.TrimSpace(out)
	dockerCmd(c, "wait", cleanedContainerID)

	out, _ = dockerCmd(c, "logs", cleanedContainerID)
	c.Assert(out, check.Equals, "foobar\n", check.Commentf("container should've printed 'foobar'"))

	dockerCmd(c, "restart", cleanedContainerID)

	out, _ = dockerCmd(c, "logs", cleanedContainerID)
	c.Assert(out, check.Equals, "foobar\nfoobar\n", check.Commentf("container should've printed 'foobar' twice, got %v", out))
}

func (s *DockerSuite) TestRestartRunningContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "sh", "-c", "echo foobar && sleep 30 && echo 'should not print this'")

	cleanedContainerID := strings.TrimSpace(out)

	c.Assert(waitRun(cleanedContainerID), check.IsNil)

	out, _ = dockerCmd(c, "logs", cleanedContainerID)
	c.Assert(out, check.Equals, "foobar\n", check.Commentf("container should've printed 'foobar'"))

	dockerCmd(c, "restart", "-t", "1", cleanedContainerID)

	out, _ = dockerCmd(c, "logs", cleanedContainerID)

	c.Assert(waitRun(cleanedContainerID), check.IsNil)

	c.Assert(out, check.Equals, "foobar\nfoobar\n", check.Commentf("container should've printed 'foobar' twice"))
}

// Test that restarting a container with a volume does not create a new volume on restart. Regression test for #819.
func (s *DockerSuite) TestRestartWithVolumes(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "-v", "/test", "busybox", "top")

	cleanedContainerID := strings.TrimSpace(out)
	out, _ = dockerCmd(c, "inspect", "--format", "{{ len .Mounts }}", cleanedContainerID)
	out = strings.Trim(out, " \n\r")
        c.Assert(out, check.Equals, "1", check.Commentf("expect 1 volume received %s", out))

	source, err := inspectMountSourceField(cleanedContainerID, "/test")
	c.Assert(err, check.IsNil)

	dockerCmd(c, "restart", cleanedContainerID)

	out, _ = dockerCmd(c, "inspect", "--format", "{{ len .Mounts }}", cleanedContainerID)

	out = strings.Trim(out, " \n\r")
        c.Assert(out, check.Equals, "1", check.Commentf("expect 1 volume after restart received %s", out))

	sourceAfterRestart, err := inspectMountSourceField(cleanedContainerID, "/test")
	c.Assert(err, check.IsNil)
	c.Assert(source, check.Equals, sourceAfterRestart, check.Commentf("expected volume path: %s Actual path: %s", source, sourceAfterRestart))
}

func (s *DockerSuite) TestRestartPolicyNO(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--restart=no", "busybox", "false")

	id := strings.TrimSpace(string(out))
	name, err := inspectField(id, "HostConfig.RestartPolicy.Name")
	c.Assert(err, check.IsNil)
	c.Assert(name, check.Equals, "no", check.Commentf("Container restart policy name is %s, expected %s", name, "no"))
}

func (s *DockerSuite) TestRestartPolicyAlways(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--restart=always", "busybox", "false")

	id := strings.TrimSpace(string(out))
	name, err := inspectField(id, "HostConfig.RestartPolicy.Name")
	c.Assert(err, check.IsNil)
	c.Assert(name, check.Equals, "always", check.Commentf("Container restart policy name is %s, expected %s", name, "always"))

	MaximumRetryCount, err := inspectField(id, "HostConfig.RestartPolicy.MaximumRetryCount")
	c.Assert(err, check.IsNil)

	// MaximumRetryCount=0 if the restart policy is always
	c.Assert(MaximumRetryCount, check.Equals, "0", check.Commentf("Container Maximum Retry Count is %s, expected %s", MaximumRetryCount, "0"))
}

func (s *DockerSuite) TestRestartPolicyOnFailure(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--restart=on-failure:1", "busybox", "false")

	id := strings.TrimSpace(string(out))
	name, err := inspectField(id, "HostConfig.RestartPolicy.Name")
	c.Assert(err, check.IsNil)
	c.Assert(name, check.Equals, "on-failure", check.Commentf("Container restart policy name is %s, expected %s", name, "on-failure"))

}

// a good container with --restart=on-failure:3
// MaximumRetryCount!=0; RestartCount=0
func (s *DockerSuite) TestContainerRestartwithGoodContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--restart=on-failure:3", "busybox", "true")

	id := strings.TrimSpace(string(out))

	err := waitInspect(id, "{{ .State.Restarting }} {{ .State.Running }}", "false false", 5*time.Second)
        c.Assert(err, check.IsNil)

	count, err := inspectField(id, "RestartCount")
	c.Assert(err, check.IsNil)
	c.Assert(count, check.Equals, "0", check.Commentf("Container was restarted %s times, expected %d", count, 0))

	MaximumRetryCount, err := inspectField(id, "HostConfig.RestartPolicy.MaximumRetryCount")
	c.Assert(err, check.IsNil)
	c.Assert(MaximumRetryCount, check.Equals, "3", check.Commentf("Container Maximum Retry Count is %s, expected %s", MaximumRetryCount, "3"))

}
