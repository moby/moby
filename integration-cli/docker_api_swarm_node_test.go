//go:build !windows

package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/daemon"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
)

func (s *DockerSwarmSuite) TestAPISwarmListNodes(c *testing.T) {
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, false)
	d3 := s.AddDaemon(c, true, false)

	nodes := d1.ListNodes(c)
	assert.Equal(c, len(nodes), 3, fmt.Sprintf("nodes: %#v", nodes))

loop0:
	for _, n := range nodes {
		for _, d := range []*daemon.Daemon{d1, d2, d3} {
			if n.ID == d.NodeID() {
				continue loop0
			}
		}
		c.Errorf("unknown nodeID %v", n.ID)
	}
}

func (s *DockerSwarmSuite) TestAPISwarmNodeUpdate(c *testing.T) {
	d := s.AddDaemon(c, true, true)

	nodes := d.ListNodes(c)

	d.UpdateNode(c, nodes[0].ID, func(n *swarm.Node) {
		n.Spec.Availability = swarm.NodeAvailabilityPause
	})

	n := d.GetNode(c, nodes[0].ID)
	assert.Equal(c, n.Spec.Availability, swarm.NodeAvailabilityPause)
}

func (s *DockerSwarmSuite) TestAPISwarmNodeRemove(c *testing.T) {
	testRequires(c, Network)
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, false)
	_ = s.AddDaemon(c, true, false)

	nodes := d1.ListNodes(c)
	assert.Equal(c, len(nodes), 3, fmt.Sprintf("nodes: %#v", nodes))

	// Getting the info so we can take the NodeID
	d2Info := d2.SwarmInfo(c)

	// forceful removal of d2 should work
	d1.RemoveNode(c, d2Info.NodeID, true)

	nodes = d1.ListNodes(c)
	assert.Equal(c, len(nodes), 2, fmt.Sprintf("nodes: %#v", nodes))

	// Restart the node that was removed
	d2.RestartNode(c)

	// Give some time for the node to rejoin
	time.Sleep(1 * time.Second)

	// Make sure the node didn't rejoin
	nodes = d1.ListNodes(c)
	assert.Equal(c, len(nodes), 2, fmt.Sprintf("nodes: %#v", nodes))
}

func (s *DockerSwarmSuite) TestAPISwarmNodeDrainPause(c *testing.T) {
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, false)

	time.Sleep(1 * time.Second) // make sure all daemons are ready to accept tasks

	// start a service, expect balanced distribution
	instances := 2
	id := d1.CreateService(c, simpleTestService, setInstances(instances))

	poll.WaitOn(c, pollCheck(c, d1.CheckActiveContainerCount, checker.GreaterThan(0)), poll.WithTimeout(defaultReconciliationTimeout))
	poll.WaitOn(c, pollCheck(c, d2.CheckActiveContainerCount, checker.GreaterThan(0)), poll.WithTimeout(defaultReconciliationTimeout))
	poll.WaitOn(c, pollCheck(c, reducedCheck(sumAsIntegers, d1.CheckActiveContainerCount, d2.CheckActiveContainerCount), checker.Equals(instances)), poll.WithTimeout(defaultReconciliationTimeout))

	// drain d2, all containers should move to d1
	d1.UpdateNode(c, d2.NodeID(), func(n *swarm.Node) {
		n.Spec.Availability = swarm.NodeAvailabilityDrain
	})
	poll.WaitOn(c, pollCheck(c, d1.CheckActiveContainerCount, checker.Equals(instances)), poll.WithTimeout(defaultReconciliationTimeout))
	poll.WaitOn(c, pollCheck(c, d2.CheckActiveContainerCount, checker.Equals(0)), poll.WithTimeout(defaultReconciliationTimeout))

	// set d2 back to active
	d1.UpdateNode(c, d2.NodeID(), func(n *swarm.Node) {
		n.Spec.Availability = swarm.NodeAvailabilityActive
	})

	instances = 1
	d1.UpdateService(c, d1.GetService(c, id), setInstances(instances))
	poll.WaitOn(c, pollCheck(c, reducedCheck(sumAsIntegers, d1.CheckActiveContainerCount, d2.CheckActiveContainerCount), checker.Equals(instances)), poll.WithTimeout(defaultReconciliationTimeout*2))

	instances = 2
	d1.UpdateService(c, d1.GetService(c, id), setInstances(instances))

	// drained node first so we don't get any old containers
	poll.WaitOn(c, pollCheck(c, d2.CheckActiveContainerCount, checker.GreaterThan(0)), poll.WithTimeout(defaultReconciliationTimeout))
	poll.WaitOn(c, pollCheck(c, d1.CheckActiveContainerCount, checker.GreaterThan(0)), poll.WithTimeout(defaultReconciliationTimeout))
	poll.WaitOn(c, pollCheck(c, reducedCheck(sumAsIntegers, d1.CheckActiveContainerCount, d2.CheckActiveContainerCount), checker.Equals(instances)), poll.WithTimeout(defaultReconciliationTimeout*2))

	d2ContainerCount := len(d2.ActiveContainers(c))

	// set d2 to paused, scale service up, only d1 gets new tasks
	d1.UpdateNode(c, d2.NodeID(), func(n *swarm.Node) {
		n.Spec.Availability = swarm.NodeAvailabilityPause
	})

	instances = 4
	d1.UpdateService(c, d1.GetService(c, id), setInstances(instances))
	poll.WaitOn(c, pollCheck(c, d1.CheckActiveContainerCount, checker.Equals(instances-d2ContainerCount)), poll.WithTimeout(defaultReconciliationTimeout))
	poll.WaitOn(c, pollCheck(c, d2.CheckActiveContainerCount, checker.Equals(d2ContainerCount)), poll.WithTimeout(defaultReconciliationTimeout))
}
