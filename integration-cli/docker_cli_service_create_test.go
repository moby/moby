// +build !windows

package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestServiceCreateMountVolume(c *check.C) {
	d := s.AddDaemon(c, true, true)
	out, err := d.Cmd("service", "create", "--mount", "type=volume,source=foo,target=/foo,volume-nocopy", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	id := strings.TrimSpace(out)

	var tasks []swarm.Task
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		tasks = d.GetServiceTasks(c, id)
		return len(tasks) > 0, nil
	}, checker.Equals, true)

	task := tasks[0]
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		if task.NodeID == "" || task.Status.ContainerStatus.ContainerID == "" {
			task = d.GetTask(c, task.ID)
		}
		return task.NodeID != "" && task.Status.ContainerStatus.ContainerID != "", nil
	}, checker.Equals, true)

	// check container mount config
	out, err = s.nodeCmd(c, task.NodeID, "inspect", "--format", "{{json .HostConfig.Mounts}}", task.Status.ContainerStatus.ContainerID)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	var mountConfig []mount.Mount
	c.Assert(json.Unmarshal([]byte(out), &mountConfig), checker.IsNil)
	c.Assert(mountConfig, checker.HasLen, 1)

	c.Assert(mountConfig[0].Source, checker.Equals, "foo")
	c.Assert(mountConfig[0].Target, checker.Equals, "/foo")
	c.Assert(mountConfig[0].Type, checker.Equals, mount.TypeVolume)
	c.Assert(mountConfig[0].VolumeOptions, checker.NotNil)
	c.Assert(mountConfig[0].VolumeOptions.NoCopy, checker.True)

	// check container mounts actual
	out, err = s.nodeCmd(c, task.NodeID, "inspect", "--format", "{{json .Mounts}}", task.Status.ContainerStatus.ContainerID)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	var mounts []types.MountPoint
	c.Assert(json.Unmarshal([]byte(out), &mounts), checker.IsNil)
	c.Assert(mounts, checker.HasLen, 1)

	c.Assert(mounts[0].Type, checker.Equals, mount.TypeVolume)
	c.Assert(mounts[0].Name, checker.Equals, "foo")
	c.Assert(mounts[0].Destination, checker.Equals, "/foo")
	c.Assert(mounts[0].RW, checker.Equals, true)
}

func (s *DockerSwarmSuite) TestServiceCreateWithSecretSimple(c *check.C) {
	d := s.AddDaemon(c, true, true)

	serviceName := "test-service-secret"
	testName := "test_secret"
	id := d.CreateSecret(c, swarm.SecretSpec{
		swarm.Annotations{
			Name: testName,
		},
		[]byte("TESTINGDATA"),
	})
	c.Assert(id, checker.Not(checker.Equals), "", check.Commentf("secrets: %s", id))

	out, err := d.Cmd("service", "create", "--name", serviceName, "--secret", testName, "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = d.Cmd("service", "inspect", "--format", "{{ json .Spec.TaskTemplate.ContainerSpec.Secrets }}", serviceName)
	c.Assert(err, checker.IsNil)

	var refs []swarm.SecretReference
	c.Assert(json.Unmarshal([]byte(out), &refs), checker.IsNil)
	c.Assert(refs, checker.HasLen, 1)

	c.Assert(refs[0].SecretName, checker.Equals, testName)
	c.Assert(refs[0].File, checker.Not(checker.IsNil))
	c.Assert(refs[0].File.Name, checker.Equals, testName)
	c.Assert(refs[0].File.UID, checker.Equals, "0")
	c.Assert(refs[0].File.GID, checker.Equals, "0")
}

func (s *DockerSwarmSuite) TestServiceCreateWithSecretSourceTarget(c *check.C) {
	d := s.AddDaemon(c, true, true)

	serviceName := "test-service-secret"
	testName := "test_secret"
	id := d.CreateSecret(c, swarm.SecretSpec{
		swarm.Annotations{
			Name: testName,
		},
		[]byte("TESTINGDATA"),
	})
	c.Assert(id, checker.Not(checker.Equals), "", check.Commentf("secrets: %s", id))
	testTarget := "testing"

	out, err := d.Cmd("service", "create", "--name", serviceName, "--secret", fmt.Sprintf("source=%s,target=%s", testName, testTarget), "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = d.Cmd("service", "inspect", "--format", "{{ json .Spec.TaskTemplate.ContainerSpec.Secrets }}", serviceName)
	c.Assert(err, checker.IsNil)

	var refs []swarm.SecretReference
	c.Assert(json.Unmarshal([]byte(out), &refs), checker.IsNil)
	c.Assert(refs, checker.HasLen, 1)

	c.Assert(refs[0].SecretName, checker.Equals, testName)
	c.Assert(refs[0].File, checker.Not(checker.IsNil))
	c.Assert(refs[0].File.Name, checker.Equals, testTarget)
}

func (s *DockerSwarmSuite) TestServiceCreateMountTmpfs(c *check.C) {
	d := s.AddDaemon(c, true, true)
	out, err := d.Cmd("service", "create", "--mount", "type=tmpfs,target=/foo,tmpfs-size=1MB", "busybox", "sh", "-c", "mount | grep foo; tail -f /dev/null")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	id := strings.TrimSpace(out)

	var tasks []swarm.Task
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		tasks = d.GetServiceTasks(c, id)
		return len(tasks) > 0, nil
	}, checker.Equals, true)

	task := tasks[0]
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		if task.NodeID == "" || task.Status.ContainerStatus.ContainerID == "" {
			task = d.GetTask(c, task.ID)
		}
		return task.NodeID != "" && task.Status.ContainerStatus.ContainerID != "", nil
	}, checker.Equals, true)

	// check container mount config
	out, err = s.nodeCmd(c, task.NodeID, "inspect", "--format", "{{json .HostConfig.Mounts}}", task.Status.ContainerStatus.ContainerID)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	var mountConfig []mount.Mount
	c.Assert(json.Unmarshal([]byte(out), &mountConfig), checker.IsNil)
	c.Assert(mountConfig, checker.HasLen, 1)

	c.Assert(mountConfig[0].Source, checker.Equals, "")
	c.Assert(mountConfig[0].Target, checker.Equals, "/foo")
	c.Assert(mountConfig[0].Type, checker.Equals, mount.TypeTmpfs)
	c.Assert(mountConfig[0].TmpfsOptions, checker.NotNil)
	c.Assert(mountConfig[0].TmpfsOptions.SizeBytes, checker.Equals, int64(1048576))

	// check container mounts actual
	out, err = s.nodeCmd(c, task.NodeID, "inspect", "--format", "{{json .Mounts}}", task.Status.ContainerStatus.ContainerID)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	var mounts []types.MountPoint
	c.Assert(json.Unmarshal([]byte(out), &mounts), checker.IsNil)
	c.Assert(mounts, checker.HasLen, 1)

	c.Assert(mounts[0].Type, checker.Equals, mount.TypeTmpfs)
	c.Assert(mounts[0].Name, checker.Equals, "")
	c.Assert(mounts[0].Destination, checker.Equals, "/foo")
	c.Assert(mounts[0].RW, checker.Equals, true)

	out, err = s.nodeCmd(c, task.NodeID, "logs", task.Status.ContainerStatus.ContainerID)
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(strings.TrimSpace(out), checker.HasPrefix, "tmpfs on /foo type tmpfs")
	c.Assert(strings.TrimSpace(out), checker.Contains, "size=1024k")
}
