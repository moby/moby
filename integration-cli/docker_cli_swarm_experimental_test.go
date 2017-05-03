// +build !windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/daemon"
	"github.com/go-check/check"
)

func introspectSwarm(c *check.C, d *daemon.Swarm, containerID string) (*types.ContainerJSON, *swarm.Task, *swarm.Service, *swarm.Node) {
	out, err := d.Cmd("inspect", containerID, "--format", "{{json .}}")
	c.Assert(err, checker.IsNil)
	var container types.ContainerJSON
	c.Assert(json.Unmarshal([]byte(out), &container), checker.IsNil)
	task := d.GetTask(c, container.Config.Labels["com.docker.swarm.task.id"])
	service := d.GetService(c, container.Config.Labels["com.docker.swarm.service.id"])
	node := d.GetNode(c, container.Config.Labels["com.docker.swarm.node.id"])
	return &container, &task, service, node
}

func testIntrospectionDaemon(c *check.C, d *daemon.Daemon, container *types.ContainerJSON, mountpoint string) {
	nameFile, err := d.Cmd("exec", container.ID, "cat", filepath.Join(mountpoint, "daemon", "name"))
	c.Assert(err, checker.IsNil)
	hostname, _ := os.Hostname()
	c.Assert(nameFile, checker.Equals, hostname+"\n")
}

func testIntrospectionContainer(c *check.C, d *daemon.Daemon, container *types.ContainerJSON, mountpoint string) {
	idFile, err := d.Cmd("exec", container.ID, "cat", filepath.Join(mountpoint, "container", "id"))
	c.Assert(err, checker.IsNil)
	c.Assert(idFile, checker.Equals, container.ID+"\n")

	nameFile, err := d.Cmd("exec", container.ID, "cat", filepath.Join(mountpoint, "container", "name"))
	c.Assert(err, checker.IsNil)
	fullnameFile, err := d.Cmd("exec", container.ID, "cat", filepath.Join(mountpoint, "container", "fullname"))
	c.Assert(err, checker.IsNil)
	c.Assert(fullnameFile, checker.Equals, "/"+nameFile)
	for k, v := range container.Config.Labels {
		f, err := d.Cmd("exec", container.ID, "cat", filepath.Join(mountpoint, "container", "labels", k))
		c.Assert(err, checker.IsNil)
		if v == "" {
			c.Assert(f, checker.Equals, "")
		} else {
			c.Assert(f, checker.Equals, v+"\n")
		}
	}
}

func testIntrospectionTask(c *check.C, d *daemon.Swarm, container *types.ContainerJSON, task *swarm.Task, mountpoint string) {
	idFile, err := d.Cmd("exec", container.ID, "cat", filepath.Join(mountpoint, "task", "id"))
	c.Assert(err, checker.IsNil)
	c.Assert(idFile, checker.Equals, task.ID+"\n")

	nameFile, err := d.Cmd("exec", container.ID, "cat", filepath.Join(mountpoint, "task", "name"))
	c.Assert(err, checker.IsNil)
	c.Assert(nameFile, checker.Equals, container.Config.Labels["com.docker.swarm.task.name"]+"\n") // not *swarm.Task.Spec.Name

	slotFile, err := d.Cmd("exec", container.ID, "cat", filepath.Join(mountpoint, "task", "slot"))
	c.Assert(err, checker.IsNil)
	c.Assert(slotFile, checker.Equals, fmt.Sprintf("%d\n", task.Slot))
	// no labels yet
}

func testIntrospectionService(c *check.C, d *daemon.Swarm, container *types.ContainerJSON, service *swarm.Service, mountpoint string) {
	idFile, err := d.Cmd("exec", container.ID, "cat", filepath.Join(mountpoint, "service", "id"))
	c.Assert(err, checker.IsNil)
	c.Assert(idFile, checker.Equals, service.ID+"\n")

	nameFile, err := d.Cmd("exec", container.ID, "cat", filepath.Join(mountpoint, "service", "name"))
	c.Assert(err, checker.IsNil)
	c.Assert(nameFile, checker.Equals, service.Spec.Name+"\n")
	// no labels yet
}

func (s *DockerSwarmSuite) TestSwarmServiceWithIntrospectionVolume(c *check.C) {
	testRequires(c, DaemonIsLinux, ExperimentalDaemon)
	d := s.AddDaemon(c, true, true)
	name := "introspection"
	replicas := 3
	mountpoint := "/foo"
	out, err := d.Cmd("service", "create", "--name", name,
		"--replicas", strconv.Itoa(replicas),
		"--mount", "type=introspection,introspection-scope=.,dst="+mountpoint,
		"busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	// make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, replicas)

	out, err = d.Cmd("ps", "-q", "--no-trunc")
	c.Assert(err, checker.IsNil)
	containers := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(containers, checker.HasLen, replicas)

	for _, containerID := range containers {
		container, task, service, _ := introspectSwarm(c, d, containerID)
		testIntrospectionDaemon(c, d.Daemon, container, mountpoint)
		testIntrospectionContainer(c, d.Daemon, container, mountpoint)
		testIntrospectionTask(c, d, container, task, mountpoint)
		testIntrospectionService(c, d, container, service, mountpoint)
	}
}
