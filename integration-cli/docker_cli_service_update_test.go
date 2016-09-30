// +build !windows

package main

import (
	"encoding/json"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestServiceUpdatePort(c *check.C) {
	d := s.AddDaemon(c, true, true)

	serviceName := "TestServiceUpdatePort"
	serviceArgs := append([]string{"service", "create", "--name", serviceName, "-p", "8080:8081", defaultSleepImage}, sleepCommandForDaemonPlatform()...)

	// Create a service with a port mapping of 8080:8081.
	out, err := d.Cmd(serviceArgs...)
	c.Assert(err, checker.IsNil)
	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 1)

	// Update the service: changed the port mapping from 8080:8081 to 8082:8083.
	_, err = d.Cmd("service", "update", "--publish-add", "8082:8083", "--publish-rm", "8081", serviceName)
	c.Assert(err, checker.IsNil)

	// Inspect the service and verify port mapping
	expected := []swarm.PortConfig{
		{
			Protocol:      "tcp",
			PublishedPort: 8082,
			TargetPort:    8083,
		},
	}

	out, err = d.Cmd("service", "inspect", "--format", "{{ json .Spec.EndpointSpec.Ports }}", serviceName)
	c.Assert(err, checker.IsNil)

	var portConfig []swarm.PortConfig
	if err := json.Unmarshal([]byte(out), &portConfig); err != nil {
		c.Fatalf("invalid JSON in inspect result: %v (%s)", err, out)
	}
	c.Assert(portConfig, checker.DeepEquals, expected)
}

func (s *DockerSwarmSuite) TestServiceUpdateLabel(c *check.C) {
	d := s.AddDaemon(c, true, true)
	out, err := d.Cmd("service", "create", "--name=test", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	service := d.getService(c, "test")
	c.Assert(service.Spec.Labels, checker.HasLen, 0)

	// add label to empty set
	out, err = d.Cmd("service", "update", "test", "--label-add", "foo=bar")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	service = d.getService(c, "test")
	c.Assert(service.Spec.Labels, checker.HasLen, 1)
	c.Assert(service.Spec.Labels["foo"], checker.Equals, "bar")

	// add label to non-empty set
	out, err = d.Cmd("service", "update", "test", "--label-add", "foo2=bar")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	service = d.getService(c, "test")
	c.Assert(service.Spec.Labels, checker.HasLen, 2)
	c.Assert(service.Spec.Labels["foo2"], checker.Equals, "bar")

	out, err = d.Cmd("service", "update", "test", "--label-rm", "foo2")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	service = d.getService(c, "test")
	c.Assert(service.Spec.Labels, checker.HasLen, 1)
	c.Assert(service.Spec.Labels["foo2"], checker.Equals, "")

	out, err = d.Cmd("service", "update", "test", "--label-rm", "foo")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	service = d.getService(c, "test")
	c.Assert(service.Spec.Labels, checker.HasLen, 0)
	c.Assert(service.Spec.Labels["foo"], checker.Equals, "")

	// now make sure we can add again
	out, err = d.Cmd("service", "update", "test", "--label-add", "foo=bar")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	service = d.getService(c, "test")
	c.Assert(service.Spec.Labels, checker.HasLen, 1)
	c.Assert(service.Spec.Labels["foo"], checker.Equals, "bar")
}
