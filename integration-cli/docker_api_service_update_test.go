// +build !windows

package main

import (
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/daemon"
	"github.com/go-check/check"
)

func setPortConfig(portConfig []swarm.PortConfig) daemon.ServiceConstructor {
	return func(s *swarm.Service) {
		if s.Spec.EndpointSpec == nil {
			s.Spec.EndpointSpec = &swarm.EndpointSpec{}
		}
		s.Spec.EndpointSpec.Ports = portConfig
	}
}

func (s *DockerSwarmSuite) TestAPIServiceUpdatePort(c *check.C) {
	d := s.AddDaemon(c, true, true)

	// Create a service with a port mapping of 8080:8081.
	portConfig := []swarm.PortConfig{{TargetPort: 8081, PublishedPort: 8080}}
	serviceID := d.CreateService(c, simpleTestService, setInstances(1), setPortConfig(portConfig))
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, 1)

	// Update the service: changed the port mapping from 8080:8081 to 8082:8083.
	updatedPortConfig := []swarm.PortConfig{{TargetPort: 8083, PublishedPort: 8082}}
	remoteService := d.GetService(c, serviceID)
	d.UpdateService(c, remoteService, setPortConfig(updatedPortConfig))

	// Inspect the service and verify port mapping.
	updatedService := d.GetService(c, serviceID)
	c.Assert(updatedService.Spec.EndpointSpec, check.NotNil)
	c.Assert(len(updatedService.Spec.EndpointSpec.Ports), check.Equals, 1)
	c.Assert(updatedService.Spec.EndpointSpec.Ports[0].TargetPort, check.Equals, uint32(8083))
	c.Assert(updatedService.Spec.EndpointSpec.Ports[0].PublishedPort, check.Equals, uint32(8082))
}
