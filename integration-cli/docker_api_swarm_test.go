// +build !windows

package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/engine-api/types/swarm"
	"github.com/go-check/check"
)

var defaultReconciliationTimeout = 30 * time.Second

func (s *DockerSwarmSuite) TestApiSwarmInit(c *check.C) {
	testRequires(c, Network)
	// todo: should find a better way to verify that components are running than /info
	d1 := s.AddDaemon(c, true, true)
	info, err := d1.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.ControlAvailable, checker.True)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)

	d2 := s.AddDaemon(c, true, false)
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.ControlAvailable, checker.False)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)

	// Leaving cluster
	c.Assert(d2.Leave(false), checker.IsNil)

	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.ControlAvailable, checker.False)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)

	c.Assert(d2.Join(swarm.JoinRequest{JoinToken: d1.joinTokens(c).Worker, RemoteAddrs: []string{d1.listenAddr}}), checker.IsNil)

	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.ControlAvailable, checker.False)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)

	// Current state restoring after restarts
	err = d1.Stop()
	c.Assert(err, checker.IsNil)
	err = d2.Stop()
	c.Assert(err, checker.IsNil)

	err = d1.Start()
	c.Assert(err, checker.IsNil)
	err = d2.Start()
	c.Assert(err, checker.IsNil)

	info, err = d1.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.ControlAvailable, checker.True)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)

	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.ControlAvailable, checker.False)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)
}

func (s *DockerSwarmSuite) TestApiSwarmJoinToken(c *check.C) {
	testRequires(c, Network)
	d1 := s.AddDaemon(c, false, false)
	c.Assert(d1.Init(swarm.InitRequest{}), checker.IsNil)

	d2 := s.AddDaemon(c, false, false)
	err := d2.Join(swarm.JoinRequest{RemoteAddrs: []string{d1.listenAddr}})
	c.Assert(err, checker.NotNil)
	c.Assert(err.Error(), checker.Contains, "join token is necessary")
	info, err := d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)

	err = d2.Join(swarm.JoinRequest{JoinToken: "foobaz", RemoteAddrs: []string{d1.listenAddr}})
	c.Assert(err, checker.NotNil)
	c.Assert(err.Error(), checker.Contains, "join token is necessary")
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)

	workerToken := d1.joinTokens(c).Worker

	c.Assert(d2.Join(swarm.JoinRequest{JoinToken: workerToken, RemoteAddrs: []string{d1.listenAddr}}), checker.IsNil)
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)
	c.Assert(d2.Leave(false), checker.IsNil)
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)

	// change tokens
	d1.rotateTokens(c)

	err = d2.Join(swarm.JoinRequest{JoinToken: workerToken, RemoteAddrs: []string{d1.listenAddr}})
	c.Assert(err, checker.NotNil)
	c.Assert(err.Error(), checker.Contains, "join token is necessary")
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)

	workerToken = d1.joinTokens(c).Worker

	c.Assert(d2.Join(swarm.JoinRequest{JoinToken: workerToken, RemoteAddrs: []string{d1.listenAddr}}), checker.IsNil)
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)
	c.Assert(d2.Leave(false), checker.IsNil)
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)

	// change spec, don't change tokens
	d1.updateSwarm(c, func(s *swarm.Spec) {})

	err = d2.Join(swarm.JoinRequest{RemoteAddrs: []string{d1.listenAddr}})
	c.Assert(err, checker.NotNil)
	c.Assert(err.Error(), checker.Contains, "join token is necessary")
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)

	c.Assert(d2.Join(swarm.JoinRequest{JoinToken: workerToken, RemoteAddrs: []string{d1.listenAddr}}), checker.IsNil)
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)
	c.Assert(d2.Leave(false), checker.IsNil)
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)
}

func (s *DockerSwarmSuite) TestApiSwarmCAHash(c *check.C) {
	testRequires(c, Network)
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, false, false)
	splitToken := strings.Split(d1.joinTokens(c).Worker, "-")
	splitToken[2] = "1kxftv4ofnc6mt30lmgipg6ngf9luhwqopfk1tz6bdmnkubg0e"
	replacementToken := strings.Join(splitToken, "-")
	err := d2.Join(swarm.JoinRequest{JoinToken: replacementToken, RemoteAddrs: []string{d1.listenAddr}})
	c.Assert(err, checker.NotNil)
	c.Assert(err.Error(), checker.Contains, "remote CA does not match fingerprint")
}

func (s *DockerSwarmSuite) TestApiSwarmPromoteDemote(c *check.C) {
	testRequires(c, Network)
	d1 := s.AddDaemon(c, false, false)
	c.Assert(d1.Init(swarm.InitRequest{}), checker.IsNil)
	d2 := s.AddDaemon(c, true, false)

	info, err := d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.ControlAvailable, checker.False)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)

	d1.updateNode(c, d2.NodeID, func(n *swarm.Node) {
		n.Spec.Role = swarm.NodeRoleManager
	})

	waitAndAssert(c, defaultReconciliationTimeout, d2.checkControlAvailable, checker.True)

	d1.updateNode(c, d2.NodeID, func(n *swarm.Node) {
		n.Spec.Role = swarm.NodeRoleWorker
	})

	waitAndAssert(c, defaultReconciliationTimeout, d2.checkControlAvailable, checker.False)

	// Demoting last node should fail
	node := d1.getNode(c, d1.NodeID)
	node.Spec.Role = swarm.NodeRoleWorker
	url := fmt.Sprintf("/nodes/%s/update?version=%d", node.ID, node.Version.Index)
	status, out, err := d1.SockRequest("POST", url, node.Spec)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusInternalServerError, check.Commentf("output: %q", string(out)))
	c.Assert(string(out), checker.Contains, "last manager of the swarm")
	info, err = d1.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)
	c.Assert(info.ControlAvailable, checker.True)

	// Promote already demoted node
	d1.updateNode(c, d2.NodeID, func(n *swarm.Node) {
		n.Spec.Role = swarm.NodeRoleManager
	})

	waitAndAssert(c, defaultReconciliationTimeout, d2.checkControlAvailable, checker.True)
}

func (s *DockerSwarmSuite) TestApiSwarmServicesEmptyList(c *check.C) {
	testRequires(c, Network)
	d := s.AddDaemon(c, true, true)

	services := d.listServices(c)
	c.Assert(services, checker.NotNil)
	c.Assert(len(services), checker.Equals, 0, check.Commentf("services: %#v", services))
}

func (s *DockerSwarmSuite) TestApiSwarmServicesCreate(c *check.C) {
	testRequires(c, Network)
	d := s.AddDaemon(c, true, true)

	instances := 2
	id := d.createService(c, simpleTestService, setInstances(instances))
	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, instances)

	service := d.getService(c, id)
	instances = 5
	d.updateService(c, service, setInstances(instances))
	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, instances)

	d.removeService(c, service.ID)
	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 0)
}

func (s *DockerSwarmSuite) TestApiSwarmServicesMultipleAgents(c *check.C) {
	testRequires(c, Network)
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, false)
	d3 := s.AddDaemon(c, true, false)

	time.Sleep(1 * time.Second) // make sure all daemons are ready to accept tasks

	instances := 9
	id := d1.createService(c, simpleTestService, setInstances(instances))

	waitAndAssert(c, defaultReconciliationTimeout, d1.checkActiveContainerCount, checker.GreaterThan, 0)
	waitAndAssert(c, defaultReconciliationTimeout, d2.checkActiveContainerCount, checker.GreaterThan, 0)
	waitAndAssert(c, defaultReconciliationTimeout, d3.checkActiveContainerCount, checker.GreaterThan, 0)

	waitAndAssert(c, defaultReconciliationTimeout, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d2.checkActiveContainerCount, d3.checkActiveContainerCount), checker.Equals, instances)

	// reconciliation on d2 node down
	c.Assert(d2.Stop(), checker.IsNil)

	waitAndAssert(c, defaultReconciliationTimeout, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d3.checkActiveContainerCount), checker.Equals, instances)

	// test downscaling
	instances = 5
	d1.updateService(c, d1.getService(c, id), setInstances(instances))
	waitAndAssert(c, defaultReconciliationTimeout, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d3.checkActiveContainerCount), checker.Equals, instances)

}

func (s *DockerSwarmSuite) TestApiSwarmServicesCreateGlobal(c *check.C) {
	testRequires(c, Network)
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, false)
	d3 := s.AddDaemon(c, true, false)

	d1.createService(c, simpleTestService, setGlobalMode)

	waitAndAssert(c, defaultReconciliationTimeout, d1.checkActiveContainerCount, checker.Equals, 1)
	waitAndAssert(c, defaultReconciliationTimeout, d2.checkActiveContainerCount, checker.Equals, 1)
	waitAndAssert(c, defaultReconciliationTimeout, d3.checkActiveContainerCount, checker.Equals, 1)

	d4 := s.AddDaemon(c, true, false)
	d5 := s.AddDaemon(c, true, false)

	waitAndAssert(c, defaultReconciliationTimeout, d4.checkActiveContainerCount, checker.Equals, 1)
	waitAndAssert(c, defaultReconciliationTimeout, d5.checkActiveContainerCount, checker.Equals, 1)
}

func (s *DockerSwarmSuite) TestApiSwarmServicesUpdate(c *check.C) {
	const nodeCount = 3
	var daemons [nodeCount]*SwarmDaemon
	for i := 0; i < nodeCount; i++ {
		daemons[i] = s.AddDaemon(c, true, i == 0)
	}
	// wait for nodes ready
	waitAndAssert(c, 5*time.Second, daemons[0].checkNodeReadyCount, checker.Equals, nodeCount)

	// service image at start
	image1 := "busybox:latest"
	// target image in update
	image2 := "busybox:test"

	// create a different tag
	for _, d := range daemons {
		out, err := d.Cmd("tag", image1, image2)
		c.Assert(err, checker.IsNil, check.Commentf(out))
	}

	// create service
	instances := 5
	parallelism := 2
	id := daemons[0].createService(c, serviceForUpdate, setInstances(instances))

	// wait for tasks ready
	waitAndAssert(c, defaultReconciliationTimeout, daemons[0].checkRunningTaskImages, checker.DeepEquals,
		map[string]int{image1: instances})

	// issue service update
	service := daemons[0].getService(c, id)
	daemons[0].updateService(c, service, setImage(image2))

	// first batch
	waitAndAssert(c, defaultReconciliationTimeout, daemons[0].checkRunningTaskImages, checker.DeepEquals,
		map[string]int{image1: instances - parallelism, image2: parallelism})

	// 2nd batch
	waitAndAssert(c, defaultReconciliationTimeout, daemons[0].checkRunningTaskImages, checker.DeepEquals,
		map[string]int{image1: instances - 2*parallelism, image2: 2 * parallelism})

	// 3nd batch
	waitAndAssert(c, defaultReconciliationTimeout, daemons[0].checkRunningTaskImages, checker.DeepEquals,
		map[string]int{image2: instances})
}

func (s *DockerSwarmSuite) TestApiSwarmServicesStateReporting(c *check.C) {
	testRequires(c, Network)
	testRequires(c, SameHostDaemon)
	testRequires(c, DaemonIsLinux)

	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, true)
	d3 := s.AddDaemon(c, true, false)

	time.Sleep(1 * time.Second) // make sure all daemons are ready to accept

	instances := 9
	d1.createService(c, simpleTestService, setInstances(instances))

	waitAndAssert(c, defaultReconciliationTimeout, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d2.checkActiveContainerCount, d3.checkActiveContainerCount), checker.Equals, instances)

	getContainers := func() map[string]*SwarmDaemon {
		m := make(map[string]*SwarmDaemon)
		for _, d := range []*SwarmDaemon{d1, d2, d3} {
			for _, id := range d.activeContainers() {
				m[id] = d
			}
		}
		return m
	}

	containers := getContainers()
	c.Assert(containers, checker.HasLen, instances)
	var toRemove string
	for i := range containers {
		toRemove = i
	}

	_, err := containers[toRemove].Cmd("stop", toRemove)
	c.Assert(err, checker.IsNil)

	waitAndAssert(c, defaultReconciliationTimeout, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d2.checkActiveContainerCount, d3.checkActiveContainerCount), checker.Equals, instances)

	containers2 := getContainers()
	c.Assert(containers2, checker.HasLen, instances)
	for i := range containers {
		if i == toRemove {
			c.Assert(containers2[i], checker.IsNil)
		} else {
			c.Assert(containers2[i], checker.NotNil)
		}
	}

	containers = containers2
	for i := range containers {
		toRemove = i
	}

	// try with killing process outside of docker
	pidStr, err := containers[toRemove].Cmd("inspect", "-f", "{{.State.Pid}}", toRemove)
	c.Assert(err, checker.IsNil)
	pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
	c.Assert(err, checker.IsNil)
	c.Assert(syscall.Kill(pid, syscall.SIGKILL), checker.IsNil)

	time.Sleep(time.Second) // give some time to handle the signal

	waitAndAssert(c, defaultReconciliationTimeout, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d2.checkActiveContainerCount, d3.checkActiveContainerCount), checker.Equals, instances)

	containers2 = getContainers()
	c.Assert(containers2, checker.HasLen, instances)
	for i := range containers {
		if i == toRemove {
			c.Assert(containers2[i], checker.IsNil)
		} else {
			c.Assert(containers2[i], checker.NotNil)
		}
	}
}

func (s *DockerSwarmSuite) TestApiSwarmLeaderElection(c *check.C) {
	// Create 3 nodes
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, true)
	d3 := s.AddDaemon(c, true, true)

	// assert that the first node we made is the leader, and the other two are followers
	c.Assert(d1.getNode(c, d1.NodeID).ManagerStatus.Leader, checker.True)
	c.Assert(d1.getNode(c, d2.NodeID).ManagerStatus.Leader, checker.False)
	c.Assert(d1.getNode(c, d3.NodeID).ManagerStatus.Leader, checker.False)

	leader := d1

	// stop the leader
	leader.Stop()

	// wait for an election to occur
	var newleader *SwarmDaemon

	for _, d := range []*SwarmDaemon{d2, d3} {
		if d.getNode(c, d.NodeID).ManagerStatus.Leader {
			newleader = d
			break
		}
	}

	// assert that we have a new leader
	c.Assert(newleader, checker.NotNil)

	// add the old leader back
	leader.Start()

	// clear leader and reinit the followers list
	followers := make([]*SwarmDaemon, 0, 3)

	// pick out the leader and the followers again
	for _, d := range []*SwarmDaemon{d1, d2, d3} {
		if d1.getNode(c, d.NodeID).ManagerStatus.Leader {
			leader = d
		} else {
			followers = append(followers, d)
		}
	}

	// verify that we still only have 1 leader and 2 followers
	c.Assert(leader, checker.NotNil)
	c.Assert(followers, checker.HasLen, 2)
	// and that after we added d1 back, the leader hasn't changed
	c.Assert(leader.NodeID, checker.Equals, newleader.NodeID)
}

func (s *DockerSwarmSuite) TestApiSwarmRaftQuorum(c *check.C) {
	testRequires(c, Network)
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, true)
	d3 := s.AddDaemon(c, true, true)

	d1.createService(c, simpleTestService)

	c.Assert(d2.Stop(), checker.IsNil)

	d1.createService(c, simpleTestService, func(s *swarm.Service) {
		s.Spec.Name = "top1"
	})

	c.Assert(d3.Stop(), checker.IsNil)

	var service swarm.Service
	simpleTestService(&service)
	service.Spec.Name = "top2"
	status, out, err := d1.SockRequest("POST", "/services/create", service.Spec)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusInternalServerError, check.Commentf("deadline exceeded", string(out)))

	c.Assert(d2.Start(), checker.IsNil)

	d1.createService(c, simpleTestService, func(s *swarm.Service) {
		s.Spec.Name = "top3"
	})
}

func (s *DockerSwarmSuite) TestApiSwarmListNodes(c *check.C) {
	testRequires(c, Network)
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, false)
	d3 := s.AddDaemon(c, true, false)

	nodes := d1.listNodes(c)
	c.Assert(len(nodes), checker.Equals, 3, check.Commentf("nodes: %#v", nodes))

loop0:
	for _, n := range nodes {
		for _, d := range []*SwarmDaemon{d1, d2, d3} {
			if n.ID == d.NodeID {
				continue loop0
			}
		}
		c.Errorf("unknown nodeID %v", n.ID)
	}
}

func (s *DockerSwarmSuite) TestApiSwarmNodeUpdate(c *check.C) {
	testRequires(c, Network)
	d := s.AddDaemon(c, true, true)

	nodes := d.listNodes(c)

	d.updateNode(c, nodes[0].ID, func(n *swarm.Node) {
		n.Spec.Availability = swarm.NodeAvailabilityPause
	})

	n := d.getNode(c, nodes[0].ID)
	c.Assert(n.Spec.Availability, checker.Equals, swarm.NodeAvailabilityPause)
}

func (s *DockerSwarmSuite) TestApiSwarmNodeDrainPause(c *check.C) {
	testRequires(c, Network)
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, false)

	time.Sleep(1 * time.Second) // make sure all daemons are ready to accept tasks

	// start a service, expect balanced distribution
	instances := 8
	id := d1.createService(c, simpleTestService, setInstances(instances))

	waitAndAssert(c, defaultReconciliationTimeout, d1.checkActiveContainerCount, checker.GreaterThan, 0)
	waitAndAssert(c, defaultReconciliationTimeout, d2.checkActiveContainerCount, checker.GreaterThan, 0)
	waitAndAssert(c, defaultReconciliationTimeout, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d2.checkActiveContainerCount), checker.Equals, instances)

	// drain d2, all containers should move to d1
	d1.updateNode(c, d2.NodeID, func(n *swarm.Node) {
		n.Spec.Availability = swarm.NodeAvailabilityDrain
	})
	waitAndAssert(c, defaultReconciliationTimeout, d1.checkActiveContainerCount, checker.Equals, instances)
	waitAndAssert(c, defaultReconciliationTimeout, d2.checkActiveContainerCount, checker.Equals, 0)

	// set d2 back to active
	d1.updateNode(c, d2.NodeID, func(n *swarm.Node) {
		n.Spec.Availability = swarm.NodeAvailabilityActive
	})

	instances = 1
	d1.updateService(c, d1.getService(c, id), setInstances(instances))

	waitAndAssert(c, defaultReconciliationTimeout*2, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d2.checkActiveContainerCount), checker.Equals, instances)

	instances = 8
	d1.updateService(c, d1.getService(c, id), setInstances(instances))

	// drained node first so we don't get any old containers
	waitAndAssert(c, defaultReconciliationTimeout, d2.checkActiveContainerCount, checker.GreaterThan, 0)
	waitAndAssert(c, defaultReconciliationTimeout, d1.checkActiveContainerCount, checker.GreaterThan, 0)
	waitAndAssert(c, defaultReconciliationTimeout*2, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d2.checkActiveContainerCount), checker.Equals, instances)

	d2ContainerCount := len(d2.activeContainers())

	// set d2 to paused, scale service up, only d1 gets new tasks
	d1.updateNode(c, d2.NodeID, func(n *swarm.Node) {
		n.Spec.Availability = swarm.NodeAvailabilityPause
	})

	instances = 14
	d1.updateService(c, d1.getService(c, id), setInstances(instances))

	waitAndAssert(c, defaultReconciliationTimeout, d1.checkActiveContainerCount, checker.Equals, instances-d2ContainerCount)
	waitAndAssert(c, defaultReconciliationTimeout, d2.checkActiveContainerCount, checker.Equals, d2ContainerCount)

}

func (s *DockerSwarmSuite) TestApiSwarmLeaveRemovesContainer(c *check.C) {
	testRequires(c, Network)
	d := s.AddDaemon(c, true, true)

	instances := 2
	d.createService(c, simpleTestService, setInstances(instances))

	id, err := d.Cmd("run", "-d", "busybox", "top")
	c.Assert(err, checker.IsNil)
	id = strings.TrimSpace(id)

	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, instances+1)

	c.Assert(d.Leave(false), checker.NotNil)
	c.Assert(d.Leave(true), checker.IsNil)

	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 1)

	id2, err := d.Cmd("ps", "-q")
	c.Assert(err, checker.IsNil)
	c.Assert(id, checker.HasPrefix, strings.TrimSpace(id2))
}

// #23629
func (s *DockerSwarmSuite) TestApiSwarmLeaveOnPendingJoin(c *check.C) {
	s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, false, false)

	id, err := d2.Cmd("run", "-d", "busybox", "top")
	c.Assert(err, checker.IsNil)
	id = strings.TrimSpace(id)

	go d2.Join(swarm.JoinRequest{
		RemoteAddrs: []string{"nosuchhost:1234"},
	})

	waitAndAssert(c, defaultReconciliationTimeout, d2.checkLocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)

	waitAndAssert(c, defaultReconciliationTimeout, d2.checkActiveContainerCount, checker.Equals, 1)

	id2, err := d2.Cmd("ps", "-q")
	c.Assert(err, checker.IsNil)
	c.Assert(id, checker.HasPrefix, strings.TrimSpace(id2))
}

// #23705
func (s *DockerSwarmSuite) TestApiSwarmRestoreOnPendingJoin(c *check.C) {
	d := s.AddDaemon(c, false, false)
	go d.Join(swarm.JoinRequest{
		RemoteAddrs: []string{"nosuchhost:1234"},
	})

	waitAndAssert(c, defaultReconciliationTimeout, d.checkLocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)

	c.Assert(d.Stop(), checker.IsNil)
	c.Assert(d.Start(), checker.IsNil)

	info, err := d.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)
}

func (s *DockerSwarmSuite) TestApiSwarmManagerRestore(c *check.C) {
	testRequires(c, Network)
	d1 := s.AddDaemon(c, true, true)

	instances := 2
	id := d1.createService(c, simpleTestService, setInstances(instances))

	d1.getService(c, id)
	d1.Stop()
	d1.Start()
	d1.getService(c, id)

	d2 := s.AddDaemon(c, true, true)
	d2.getService(c, id)
	d2.Stop()
	d2.Start()
	d2.getService(c, id)

	d3 := s.AddDaemon(c, true, true)
	d3.getService(c, id)
	d3.Stop()
	d3.Start()
	d3.getService(c, id)

	d3.Kill()
	time.Sleep(1 * time.Second) // time to handle signal
	d3.Start()
	d3.getService(c, id)
}

func (s *DockerSwarmSuite) TestApiSwarmScaleNoRollingUpdate(c *check.C) {
	testRequires(c, Network)
	d := s.AddDaemon(c, true, true)

	instances := 2
	id := d.createService(c, simpleTestService, setInstances(instances))

	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, instances)
	containers := d.activeContainers()
	instances = 4
	d.updateService(c, d.getService(c, id), setInstances(instances))
	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, instances)
	containers2 := d.activeContainers()

loop0:
	for _, c1 := range containers {
		for _, c2 := range containers2 {
			if c1 == c2 {
				continue loop0
			}
		}
		c.Errorf("container %v not found in new set %#v", c1, containers2)
	}
}

func (s *DockerSwarmSuite) TestApiSwarmInvalidAddress(c *check.C) {
	d := s.AddDaemon(c, false, false)
	req := swarm.InitRequest{
		ListenAddr: "",
	}
	status, _, err := d.SockRequest("POST", "/swarm/init", req)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusInternalServerError)

	req2 := swarm.JoinRequest{
		ListenAddr:  "0.0.0.0:2377",
		RemoteAddrs: []string{""},
	}
	status, _, err = d.SockRequest("POST", "/swarm/join", req2)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusInternalServerError)
}

func (s *DockerSwarmSuite) TestApiSwarmForceNewCluster(c *check.C) {
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, true)

	instances := 2
	id := d1.createService(c, simpleTestService, setInstances(instances))
	waitAndAssert(c, defaultReconciliationTimeout, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d2.checkActiveContainerCount), checker.Equals, instances)

	c.Assert(d2.Stop(), checker.IsNil)

	time.Sleep(5 * time.Second)

	c.Assert(d1.Init(swarm.InitRequest{
		ForceNewCluster: true,
		Spec:            swarm.Spec{},
	}), checker.IsNil)

	waitAndAssert(c, defaultReconciliationTimeout, d1.checkActiveContainerCount, checker.Equals, instances)

	d3 := s.AddDaemon(c, true, true)
	info, err := d3.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.ControlAvailable, checker.True)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)

	instances = 4
	d3.updateService(c, d3.getService(c, id), setInstances(instances))

	waitAndAssert(c, defaultReconciliationTimeout, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d3.checkActiveContainerCount), checker.Equals, instances)
}

func simpleTestService(s *swarm.Service) {
	var ureplicas uint64
	ureplicas = 1
	s.Spec = swarm.ServiceSpec{
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: swarm.ContainerSpec{
				Image:   "busybox:latest",
				Command: []string{"/bin/top"},
			},
		},
		Mode: swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: &ureplicas,
			},
		},
	}
	s.Spec.Name = "top"
}

func serviceForUpdate(s *swarm.Service) {
	var ureplicas uint64
	ureplicas = 1
	s.Spec = swarm.ServiceSpec{
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: swarm.ContainerSpec{
				Image:   "busybox:latest",
				Command: []string{"/bin/top"},
			},
		},
		Mode: swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: &ureplicas,
			},
		},
		UpdateConfig: &swarm.UpdateConfig{
			Parallelism:   2,
			Delay:         8 * time.Second,
			FailureAction: swarm.UpdateFailureActionContinue,
		},
	}
	s.Spec.Name = "updatetest"
}

func setInstances(replicas int) serviceConstructor {
	ureplicas := uint64(replicas)
	return func(s *swarm.Service) {
		s.Spec.Mode = swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: &ureplicas,
			},
		}
	}
}

func setImage(image string) serviceConstructor {
	return func(s *swarm.Service) {
		s.Spec.TaskTemplate.ContainerSpec.Image = image
	}
}

func setGlobalMode(s *swarm.Service) {
	s.Spec.Mode = swarm.ServiceMode{
		Global: &swarm.GlobalService{},
	}
}

func checkClusterHealth(c *check.C, cl []*SwarmDaemon, managerCount, workerCount int) {
	var totalMCount, totalWCount int
	for _, d := range cl {
		info, err := d.info()
		c.Assert(err, check.IsNil)
		if !info.ControlAvailable {
			totalWCount++
			continue
		}
		var leaderFound bool
		totalMCount++
		var mCount, wCount int
		for _, n := range d.listNodes(c) {
			c.Assert(n.Status.State, checker.Equals, swarm.NodeStateReady, check.Commentf("state of node %s, reported by %s", n.ID, d.Info.NodeID))
			c.Assert(n.Spec.Availability, checker.Equals, swarm.NodeAvailabilityActive, check.Commentf("availability of node %s, reported by %s", n.ID, d.Info.NodeID))
			if n.Spec.Role == swarm.NodeRoleManager {
				c.Assert(n.ManagerStatus, checker.NotNil, check.Commentf("manager status of node %s (manager), reported by %s", n.ID, d.Info.NodeID))
				if n.ManagerStatus.Leader {
					leaderFound = true
				}
				mCount++
			} else {
				c.Assert(n.ManagerStatus, checker.IsNil, check.Commentf("manager status of node %s (worker), reported by %s", n.ID, d.Info.NodeID))
				wCount++
			}
		}
		c.Assert(leaderFound, checker.True, check.Commentf("lack of leader reported by node %s", info.NodeID))
		c.Assert(mCount, checker.Equals, managerCount, check.Commentf("managers count reported by node %s", info.NodeID))
		c.Assert(wCount, checker.Equals, workerCount, check.Commentf("workers count reported by node %s", info.NodeID))
	}
	c.Assert(totalMCount, checker.Equals, managerCount)
	c.Assert(totalWCount, checker.Equals, workerCount)
}

func (s *DockerSwarmSuite) TestApiSwarmRestartCluster(c *check.C) {
	mCount, wCount := 5, 1

	var nodes []*SwarmDaemon
	for i := 0; i < mCount; i++ {
		manager := s.AddDaemon(c, true, true)
		info, err := manager.info()
		c.Assert(err, checker.IsNil)
		c.Assert(info.ControlAvailable, checker.True)
		c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)
		nodes = append(nodes, manager)
	}

	for i := 0; i < wCount; i++ {
		worker := s.AddDaemon(c, true, false)
		info, err := worker.info()
		c.Assert(err, checker.IsNil)
		c.Assert(info.ControlAvailable, checker.False)
		c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)
		nodes = append(nodes, worker)
	}

	// stop whole cluster
	{
		var wg sync.WaitGroup
		wg.Add(len(nodes))
		errs := make(chan error, len(nodes))

		for _, d := range nodes {
			go func(daemon *SwarmDaemon) {
				defer wg.Done()
				if err := daemon.Stop(); err != nil {
					errs <- err
				}
				if root := os.Getenv("DOCKER_REMAP_ROOT"); root != "" {
					daemon.root = filepath.Dir(daemon.root)
				}
			}(d)
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			c.Assert(err, check.IsNil)
		}
	}

	// start whole cluster
	{
		var wg sync.WaitGroup
		wg.Add(len(nodes))
		errs := make(chan error, len(nodes))

		for _, d := range nodes {
			go func(daemon *SwarmDaemon) {
				defer wg.Done()
				if err := daemon.Start("--iptables=false"); err != nil {
					errs <- err
				}
			}(d)
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			c.Assert(err, check.IsNil)
		}
	}

	checkClusterHealth(c, nodes, mCount, wCount)
}
