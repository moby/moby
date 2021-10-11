//go:build !windows
// +build !windows

package main

import (
	"strconv"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/daemon/cluster/executor/container"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/cli/build"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/icmd"
	"gotest.tools/v3/poll"
)

// start a service, and then make its task unhealthy during running
// finally, unhealthy task should be detected and killed
func (s *DockerSwarmSuite) TestServiceHealthRun(c *testing.T) {
	testRequires(c, DaemonIsLinux) // busybox doesn't work on Windows

	d := s.AddDaemon(c, true, true)

	// build image with health-check
	imageName := "testhealth"
	result := cli.BuildCmd(c, imageName, cli.Daemon(d),
		build.WithDockerfile(`FROM busybox
		RUN touch /status
		HEALTHCHECK --interval=1s --timeout=5s --retries=1\
		  CMD cat /status`),
	)
	result.Assert(c, icmd.Success)

	serviceName := "healthServiceRun"
	out, err := d.Cmd("service", "create", "--no-resolve-image", "--detach=true", "--name", serviceName, imageName, "top")
	assert.NilError(c, err, out)
	id := strings.TrimSpace(out)

	var tasks []swarm.Task
	poll.WaitOn(c, pollCheck(c, func(c *testing.T) (interface{}, string) {
		tasks = d.GetServiceTasks(c, id)
		return tasks, ""
	}, checker.HasLen(1)), poll.WithTimeout(defaultReconciliationTimeout))

	task := tasks[0]

	// wait for task to start
	poll.WaitOn(c, pollCheck(c, func(c *testing.T) (interface{}, string) {
		task = d.GetTask(c, task.ID)
		return task.Status.State, ""
	}, checker.Equals(swarm.TaskStateRunning)), poll.WithTimeout(defaultReconciliationTimeout))

	containerID := task.Status.ContainerStatus.ContainerID

	// wait for container to be healthy
	poll.WaitOn(c, pollCheck(c, func(c *testing.T) (interface{}, string) {
		out, _ := d.Cmd("inspect", "--format={{.State.Health.Status}}", containerID)
		return strings.TrimSpace(out), ""
	}, checker.Equals("healthy")), poll.WithTimeout(defaultReconciliationTimeout))

	// make it fail
	d.Cmd("exec", containerID, "rm", "/status")
	// wait for container to be unhealthy
	poll.WaitOn(c, pollCheck(c, func(c *testing.T) (interface{}, string) {
		out, _ := d.Cmd("inspect", "--format={{.State.Health.Status}}", containerID)
		return strings.TrimSpace(out), ""
	}, checker.Equals("unhealthy")), poll.WithTimeout(defaultReconciliationTimeout))

	// Task should be terminated
	poll.WaitOn(c, pollCheck(c, func(c *testing.T) (interface{}, string) {
		task = d.GetTask(c, task.ID)
		return task.Status.State, ""
	}, checker.Equals(swarm.TaskStateFailed)), poll.WithTimeout(defaultReconciliationTimeout))

	if !strings.Contains(task.Status.Err, container.ErrContainerUnhealthy.Error()) {
		c.Fatal("unhealthy task exits because of other error")
	}
}

// start a service whose task is unhealthy at beginning
// its tasks should be blocked in starting stage, until health check is passed
func (s *DockerSwarmSuite) TestServiceHealthStart(c *testing.T) {
	testRequires(c, DaemonIsLinux) // busybox doesn't work on Windows

	d := s.AddDaemon(c, true, true)

	// service started from this image won't pass health check
	imageName := "testhealth"
	result := cli.BuildCmd(c, imageName, cli.Daemon(d),
		build.WithDockerfile(`FROM busybox
		HEALTHCHECK --interval=1s --timeout=1s --retries=1024\
		  CMD cat /status`),
	)
	result.Assert(c, icmd.Success)

	serviceName := "healthServiceStart"
	out, err := d.Cmd("service", "create", "--no-resolve-image", "--detach=true", "--name", serviceName, imageName, "top")
	assert.NilError(c, err, out)
	id := strings.TrimSpace(out)

	var tasks []swarm.Task
	poll.WaitOn(c, pollCheck(c, func(c *testing.T) (interface{}, string) {
		tasks = d.GetServiceTasks(c, id)
		return tasks, ""
	}, checker.HasLen(1)), poll.WithTimeout(defaultReconciliationTimeout))

	task := tasks[0]

	// wait for task to start
	poll.WaitOn(c, pollCheck(c, func(c *testing.T) (interface{}, string) {
		task = d.GetTask(c, task.ID)
		return task.Status.State, ""
	}, checker.Equals(swarm.TaskStateStarting)), poll.WithTimeout(defaultReconciliationTimeout))

	containerID := task.Status.ContainerStatus.ContainerID

	// wait for health check to work
	poll.WaitOn(c, pollCheck(c, func(c *testing.T) (interface{}, string) {
		out, _ := d.Cmd("inspect", "--format={{.State.Health.FailingStreak}}", containerID)
		failingStreak, _ := strconv.Atoi(strings.TrimSpace(out))
		return failingStreak, ""
	}, checker.GreaterThan(0)), poll.WithTimeout(defaultReconciliationTimeout))

	// task should be blocked at starting status
	task = d.GetTask(c, task.ID)
	assert.Equal(c, task.Status.State, swarm.TaskStateStarting)

	// make it healthy
	d.Cmd("exec", containerID, "touch", "/status")

	// Task should be at running status
	poll.WaitOn(c, pollCheck(c, func(c *testing.T) (interface{}, string) {
		task = d.GetTask(c, task.ID)
		return task.Status.State, ""
	}, checker.Equals(swarm.TaskStateRunning)), poll.WithTimeout(defaultReconciliationTimeout))

}
