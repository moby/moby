// +build !windows

package main

import (
	"fmt"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestServiceRemoveReplica(c *check.C) {
	d := s.AddDaemon(c, true, true)

	service1Name := "TestService1"
	_, err := d.Cmd("service", "create", "--no-resolve-image", "--detach=true", "--name", service1Name, "--replicas=3", "busybox", "top")
	c.Assert(err, checker.IsNil)
	service := d.GetService(c, service1Name)

	var tasks []swarm.Task
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		tasks = d.GetServiceTasks(c, service.ID)
		return len(tasks) > 0, nil
	}, checker.Equals, true)
	c.Assert(tasks, checker.HasLen, 3)
	
	task := tasks[0]
	taskName := fmt.Sprintf("%v.%v", service1Name, task.Slot)
	_, err = d.Cmd("service", "rm-replica", taskName)
	c.Assert(err, checker.IsNil)

	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		tasks = d.GetServiceTasks(c, service.ID)
		return len(tasks) > 0, nil
	}, checker.Equals, true)
	c.Assert(tasks, checker.HasLen, 2)
	c.Assert(tasks[0].ID, checker.Not(checker.Equals), task.ID)
	c.Assert(tasks[1].ID, checker.Not(checker.Equals), task.ID)

	task = tasks[0]
	taskName = fmt.Sprintf("%v.%v", service1Name, task.Slot)
	_, err = d.Cmd("service", "rm-replica", taskName)
	c.Assert(err, checker.IsNil)

	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		tasks = d.GetServiceTasks(c, service.ID)
		return len(tasks) > 0, nil
	}, checker.Equals, true)
	c.Assert(tasks, checker.HasLen, 1)
	c.Assert(tasks[0].ID, checker.Not(checker.Equals), task.ID)

	service2Name := "TestService2"
	_, err = d.Cmd("service", "create", "--no-resolve-image", "--detach=true", "--name", service2Name, "--mode=global", "busybox", "top")
	c.Assert(err, checker.IsNil)
	service = d.GetService(c, service2Name)

	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		tasks = d.GetServiceTasks(c, service.ID)
		return len(tasks) > 0, nil
	}, checker.Equals, true)
	
	task = tasks[0]
	taskName = fmt.Sprintf("%v.%v", service2Name, task.Slot)
	_, err = d.Cmd("service", "rm-replica", taskName)
	c.Assert(err, checker.NotNil)
}
