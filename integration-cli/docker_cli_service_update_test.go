// +build !windows

package main

import (
	"encoding/json"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/engine-api/types/swarm"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestServiceUpdatePort(c *check.C) {
	d := s.AddDaemon(c, true, true)

	serviceName := "TestServiceUpdatePort"
	serviceArgs := append([]string{"create", "--name", serviceName, "-p", "8080:8081", defaultSleepImage}, defaultSleepCommand...)

	// Create a service with a port mapping of 8080:8081.
	out, err := d.Cmd("service", serviceArgs...)
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
