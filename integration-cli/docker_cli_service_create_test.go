// +build !windows

package main

import (
	"encoding/json"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestServiceCreateMountVolume(c *check.C) {
	d := s.AddDaemon(c, true, true)
	out, err := d.Cmd("service", "create", "--mount", "type=volume,source=foo,target=/foo", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	id := strings.TrimSpace(out)

	var tasks []swarm.Task
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		tasks = d.getServiceTasks(c, id)
		return len(tasks) > 0, nil
	}, checker.Equals, true)

	task := tasks[0]
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		if task.NodeID == "" || task.Status.ContainerStatus.ContainerID == "" {
			task = d.getTask(c, task.ID)
		}
		return task.NodeID != "" && task.Status.ContainerStatus.ContainerID != "", nil
	}, checker.Equals, true)

	out, err = s.nodeCmd(c, task.NodeID, "inspect", "--format", "{{json .Mounts}}", task.Status.ContainerStatus.ContainerID)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	var mounts []types.MountPoint
	c.Assert(json.Unmarshal([]byte(out), &mounts), checker.IsNil)
	c.Assert(mounts, checker.HasLen, 1)

	c.Assert(mounts[0].Name, checker.Equals, "foo")
	c.Assert(mounts[0].Destination, checker.Equals, "/foo")
	c.Assert(mounts[0].RW, checker.Equals, true)
}
