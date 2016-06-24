// +build !windows

package main

import (
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/engine-api/types/swarm"
	"github.com/go-check/check"
)

var defaultReconciliationTimeout = 30 * time.Second

func (s *DockerSwarmSuite) TestApiSwarmInit(c *check.C) {
	// todo: should find a better way to verify that components are running than /info
	d1 := s.AddDaemon(c, true, true)
	info, err := d1.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.ControlAvailable, checker.Equals, true)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)

	d2 := s.AddDaemon(c, true, false)
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.ControlAvailable, checker.Equals, false)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)

	// Leaving cluster
	c.Assert(d2.Leave(false), checker.IsNil)

	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.ControlAvailable, checker.Equals, false)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)

	c.Assert(d2.Join(d1.listenAddr, "", "", false), checker.IsNil)

	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.ControlAvailable, checker.Equals, false)
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
	c.Assert(info.ControlAvailable, checker.Equals, true)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)

	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.ControlAvailable, checker.Equals, false)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)
}

func (s *DockerSwarmSuite) TestApiSwarmManualAcceptance(c *check.C) {
	s.testAPISwarmManualAcceptance(c, "")
}
func (s *DockerSwarmSuite) TestApiSwarmManualAcceptanceSecret(c *check.C) {
	s.testAPISwarmManualAcceptance(c, "foobaz")
}

func (s *DockerSwarmSuite) testAPISwarmManualAcceptance(c *check.C, secret string) {
	d1 := s.AddDaemon(c, false, false)
	c.Assert(d1.Init(map[string]bool{}, secret), checker.IsNil)

	d2 := s.AddDaemon(c, false, false)
	err := d2.Join(d1.listenAddr, "", "", false)
	c.Assert(err, checker.NotNil)
	if secret == "" {
		c.Assert(err.Error(), checker.Contains, "needs to be accepted")
		info, err := d2.info()
		c.Assert(err, checker.IsNil)
		c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStatePending)
		c.Assert(d2.Leave(false), checker.IsNil)
		info, err = d2.info()
		c.Assert(err, checker.IsNil)
		c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)
	} else {
		c.Assert(err.Error(), checker.Contains, "valid secret token is necessary")
		info, err := d2.info()
		c.Assert(err, checker.IsNil)
		c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)
	}
	d3 := s.AddDaemon(c, false, false)
	c.Assert(d3.Join(d1.listenAddr, secret, "", false), checker.NotNil)
	info, err := d3.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStatePending)
	c.Assert(len(info.NodeID), checker.GreaterThan, 5)
	d1.updateNode(c, info.NodeID, func(n *swarm.Node) {
		n.Spec.Membership = swarm.NodeMembershipAccepted
	})
	for i := 0; ; i++ {
		info, err := d3.info()
		c.Assert(err, checker.IsNil)
		if info.LocalNodeState == swarm.LocalNodeStateActive {
			break
		}
		if i > 100 {
			c.Fatalf("node did not become active")
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (s *DockerSwarmSuite) TestApiSwarmSecretAcceptance(c *check.C) {
	d1 := s.AddDaemon(c, false, false)
	aa := make(map[string]bool)
	aa["worker"] = true
	c.Assert(d1.Init(aa, "foobar"), checker.IsNil)

	d2 := s.AddDaemon(c, false, false)
	err := d2.Join(d1.listenAddr, "", "", false)
	c.Assert(err, checker.NotNil)
	c.Assert(err.Error(), checker.Contains, "secret token is necessary")
	info, err := d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)

	err = d2.Join(d1.listenAddr, "foobaz", "", false)
	c.Assert(err, checker.NotNil)
	c.Assert(err.Error(), checker.Contains, "secret token is necessary")
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)

	c.Assert(d2.Join(d1.listenAddr, "foobar", "", false), checker.IsNil)
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)
	c.Assert(d2.Leave(false), checker.IsNil)
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)

	// change secret
	d1.updateSwarm(c, func(s *swarm.Spec) {
		for i := range s.AcceptancePolicy.Policies {
			p := "foobaz"
			s.AcceptancePolicy.Policies[i].Secret = &p
		}
	})

	err = d2.Join(d1.listenAddr, "foobar", "", false)
	c.Assert(err, checker.NotNil)
	c.Assert(err.Error(), checker.Contains, "secret token is necessary")
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)

	c.Assert(d2.Join(d1.listenAddr, "foobaz", "", false), checker.IsNil)
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)
	c.Assert(d2.Leave(false), checker.IsNil)
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)

	// change policy, don't change secret
	d1.updateSwarm(c, func(s *swarm.Spec) {
		for i, p := range s.AcceptancePolicy.Policies {
			if p.Role == swarm.NodeRoleManager {
				s.AcceptancePolicy.Policies[i].Autoaccept = false
			}
			s.AcceptancePolicy.Policies[i].Secret = nil
		}
	})

	err = d2.Join(d1.listenAddr, "", "", false)
	c.Assert(err, checker.NotNil)
	c.Assert(err.Error(), checker.Contains, "secret token is necessary")
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)

	c.Assert(d2.Join(d1.listenAddr, "foobaz", "", false), checker.IsNil)
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)
	c.Assert(d2.Leave(false), checker.IsNil)
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)

	// clear secret
	d1.updateSwarm(c, func(s *swarm.Spec) {
		for i := range s.AcceptancePolicy.Policies {
			p := ""
			s.AcceptancePolicy.Policies[i].Secret = &p
		}
	})

	c.Assert(d2.Join(d1.listenAddr, "", "", false), checker.IsNil)
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)
	c.Assert(d2.Leave(false), checker.IsNil)
	info, err = d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)

}

func (s *DockerSwarmSuite) TestApiSwarmCAHash(c *check.C) {
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, false, false)
	err := d2.Join(d1.listenAddr, "", "foobar", false)
	c.Assert(err, checker.NotNil)
	c.Assert(err.Error(), checker.Contains, "invalid checksum digest format")

	c.Assert(len(d1.CACertHash), checker.GreaterThan, 0)
	c.Assert(d2.Join(d1.listenAddr, "", d1.CACertHash, false), checker.IsNil)
}

func (s *DockerSwarmSuite) TestApiSwarmPromoteDemote(c *check.C) {
	d1 := s.AddDaemon(c, false, false)
	c.Assert(d1.Init(map[string]bool{"worker": true}, ""), checker.IsNil)
	d2 := s.AddDaemon(c, true, false)

	info, err := d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.ControlAvailable, checker.Equals, false)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)

	d1.updateNode(c, d2.NodeID, func(n *swarm.Node) {
		n.Spec.Role = swarm.NodeRoleManager
	})

	for i := 0; ; i++ {
		info, err := d2.info()
		c.Assert(err, checker.IsNil)
		c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)
		if info.ControlAvailable {
			break
		}
		if i > 100 {
			c.Errorf("node did not turn into manager")
		} else {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	d1.updateNode(c, d2.NodeID, func(n *swarm.Node) {
		n.Spec.Role = swarm.NodeRoleWorker
	})

	for i := 0; ; i++ {
		info, err := d2.info()
		c.Assert(err, checker.IsNil)
		c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)
		if !info.ControlAvailable {
			break
		}
		if i > 100 {
			c.Errorf("node did not turn into manager")
		} else {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// todo: test raft qourum stability
}

func (s *DockerSwarmSuite) TestApiSwarmServicesCreate(c *check.C) {
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

func (s *DockerSwarmSuite) TestApiSwarmServicesStateReporting(c *check.C) {
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

func (s *DockerSwarmSuite) TestApiSwarmRaftQuorum(c *check.C) {
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
	d := s.AddDaemon(c, true, true)

	nodes := d.listNodes(c)

	d.updateNode(c, nodes[0].ID, func(n *swarm.Node) {
		n.Spec.Availability = swarm.NodeAvailabilityPause
	})

	n := d.getNode(c, nodes[0].ID)
	c.Assert(n.Spec.Availability, checker.Equals, swarm.NodeAvailabilityPause)
}

func (s *DockerSwarmSuite) TestApiSwarmNodeDrainPause(c *check.C) {
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

	go d2.Join("nosuchhost:1234", "", "", false) // will block on pending state

	for i := 0; ; i++ {
		info, err := d2.info()
		c.Assert(err, checker.IsNil)
		if info.LocalNodeState == swarm.LocalNodeStatePending {
			break
		}
		if i > 100 {
			c.Fatalf("node did not go to pending state: %v", info.LocalNodeState)
		}
		time.Sleep(100 * time.Millisecond)
	}

	c.Assert(d2.Leave(true), checker.IsNil)

	waitAndAssert(c, defaultReconciliationTimeout, d2.checkActiveContainerCount, checker.Equals, 1)

	id2, err := d2.Cmd("ps", "-q")
	c.Assert(err, checker.IsNil)
	c.Assert(id, checker.HasPrefix, strings.TrimSpace(id2))
}

// #23705
func (s *DockerSwarmSuite) TestApiSwarmRestoreOnPendingJoin(c *check.C) {
	d := s.AddDaemon(c, false, false)
	go d.Join("nosuchhost:1234", "", "", false) // will block on pending state

	for i := 0; ; i++ {
		info, err := d.info()
		c.Assert(err, checker.IsNil)
		if info.LocalNodeState == swarm.LocalNodeStatePending {
			break
		}
		if i > 100 {
			c.Fatalf("node did not go to pending state: %v", info.LocalNodeState)
		}
		time.Sleep(100 * time.Millisecond)
	}

	c.Assert(d.Stop(), checker.IsNil)
	c.Assert(d.Start(), checker.IsNil)

	info, err := d.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)
}

func (s *DockerSwarmSuite) TestApiSwarmManagerRestore(c *check.C) {
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

func setGlobalMode(s *swarm.Service) {
	s.Spec.Mode = swarm.ServiceMode{
		Global: &swarm.GlobalService{},
	}
}
