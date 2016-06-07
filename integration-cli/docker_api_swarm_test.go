// +build !windows

package main

import (
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/engine-api/types/swarm"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestApiSwarmInit(c *check.C) {
	// todo: should find a better way to verify that components are running than /info
	d1 := s.AddDaemon(c, true, true)
	ismanager, isagent, err := d1.SwarmStatus()
	c.Assert(err, checker.IsNil)
	c.Assert(ismanager, checker.Equals, true)
	c.Assert(isagent, checker.Equals, true)

	d2 := s.AddDaemon(c, true, false)
	ismanager, isagent, err = d2.SwarmStatus()
	c.Assert(err, checker.IsNil)
	c.Assert(ismanager, checker.Equals, false)
	c.Assert(isagent, checker.Equals, true)

	// Leaving cluster
	c.Assert(d2.Leave(false), checker.IsNil)

	ismanager, isagent, err = d2.SwarmStatus()
	c.Assert(err, checker.IsNil)
	c.Assert(ismanager, checker.Equals, false)
	c.Assert(isagent, checker.Equals, false)

	c.Assert(d2.Join(d1.listenAddr, "", false), checker.IsNil)

	ismanager, isagent, err = d2.SwarmStatus()
	c.Assert(err, checker.IsNil)
	c.Assert(ismanager, checker.Equals, false)
	c.Assert(isagent, checker.Equals, true)

	// Current state restoring after restarts
	err = d1.Stop()
	c.Assert(err, checker.IsNil)
	err = d2.Stop()
	c.Assert(err, checker.IsNil)

	err = d1.Start()
	c.Assert(err, checker.IsNil)
	err = d2.Start()
	c.Assert(err, checker.IsNil)

	ismanager, isagent, err = d1.SwarmStatus()
	c.Assert(err, checker.IsNil)
	c.Assert(ismanager, checker.Equals, true)
	c.Assert(isagent, checker.Equals, true)

	ismanager, isagent, err = d2.SwarmStatus()
	c.Assert(err, checker.IsNil)
	c.Assert(ismanager, checker.Equals, false)
	c.Assert(isagent, checker.Equals, true)
}

func (s *DockerSwarmSuite) TestApiSwarmManualAcceptance(c *check.C) {
	d1 := s.AddDaemon(c, false, false)
	aa := make(map[string]bool)
	c.Assert(d1.Init(aa, ""), checker.IsNil)
	time.Sleep(200 * time.Millisecond) // TODO: fix cluster update race

	d2 := s.AddDaemon(c, false, false)
	err := d2.Join(d1.listenAddr, "", false)
	c.Assert(err, checker.NotNil)
	c.Assert(err.Error(), checker.Contains, "Timeout reached")
	c.Assert(d2.Leave(false), checker.IsNil)

	// TODO: manual accpetance testing
	// d3 := s.AddDaemon(c, false, false)
	// go func() {
	// 	for {
	// 		sw, err := d3.swarmInfo()
	// 		c.Logf("sw %#v %v", sw, err)
	// 		time.Sleep(300 * time.Millisecond)
	// 	}
	// }()
	// c.Assert(d3.Join(d1.listenAddr, "", false), checker.NotNil)

	// accpet workers but not managers
	d1 = s.AddDaemon(c, false, false)
	aa["worker"] = true
	c.Assert(d1.Init(aa, ""), checker.IsNil)

	d2 = s.AddDaemon(c, false, false)
	c.Assert(d2.Join(d1.listenAddr, "", false), checker.IsNil)

	d3 := s.AddDaemon(c, false, false)
	err = d3.Join(d1.listenAddr, "", true)
	c.Assert(err, checker.NotNil)
	c.Assert(err.Error(), checker.Contains, "Timeout reached")
}

func (s *DockerSwarmSuite) TestApiSwarmSecretAcceptance(c *check.C) {
	d1 := s.AddDaemon(c, false, false)
	aa := make(map[string]bool)
	aa["worker"] = true
	c.Assert(d1.Init(aa, "foobar"), checker.IsNil)

	d2 := s.AddDaemon(c, false, false)
	err := d2.Join(d1.listenAddr, "", false)
	c.Assert(err, checker.NotNil)
	c.Assert(err.Error(), checker.Contains, "secret token is necessary")

	c.Assert(d2.Join(d1.listenAddr, "foobaz", false), checker.NotNil)
	c.Assert(err.Error(), checker.Contains, "secret token is necessary")

	c.Assert(d2.Join(d1.listenAddr, "foobar", false), checker.IsNil)
	c.Assert(d2.Leave(false), checker.IsNil)
}

func (s *DockerSwarmSuite) TestApiSwarmServicesCreate(c *check.C) {
	d := s.AddDaemon(c, true, true)

	instances := 2
	id := d.createService(c, simpleTestService, setInstances(instances))
	waitAndAssert(c, 10*time.Second, d.checkActiveContainerCount, checker.Equals, instances)

	service := d.getService(c, id)
	instances = 5
	d.updateService(c, service, setInstances(instances))
	waitAndAssert(c, 10*time.Second, d.checkActiveContainerCount, checker.Equals, instances)

	d.removeService(c, service.ID)
	waitAndAssert(c, 10*time.Second, d.checkActiveContainerCount, checker.Equals, 0)
}

func (s *DockerSwarmSuite) TestApiSwarmServicesMultipleAgents(c *check.C) {
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, false)
	d3 := s.AddDaemon(c, true, false)

	time.Sleep(1 * time.Second) // make sure all daemons are ready to accept tasks

	instances := 9
	id := d1.createService(c, simpleTestService, setInstances(instances))

	waitAndAssert(c, 10*time.Second, d1.checkActiveContainerCount, checker.GreaterThan, 0)
	waitAndAssert(c, 10*time.Second, d2.checkActiveContainerCount, checker.GreaterThan, 0)
	waitAndAssert(c, 10*time.Second, d3.checkActiveContainerCount, checker.GreaterThan, 0)

	waitAndAssert(c, 10*time.Second, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d2.checkActiveContainerCount, d3.checkActiveContainerCount), checker.Equals, instances)

	// reconciliation on d2 node down
	c.Assert(d2.Stop(), checker.IsNil)

	waitAndAssert(c, 20*time.Second, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d3.checkActiveContainerCount), checker.Equals, instances)

	// test downscaling
	instances = 5
	d1.updateService(c, d1.getService(c, id), setInstances(instances))
	waitAndAssert(c, 20*time.Second, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d3.checkActiveContainerCount), checker.Equals, instances)

}

func (s *DockerSwarmSuite) TestApiSwarmServicesCreateGlobal(c *check.C) {
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, false)
	d3 := s.AddDaemon(c, true, false)

	d1.createService(c, simpleTestService, setGlobalMode)

	waitAndAssert(c, 10*time.Second, d1.checkActiveContainerCount, checker.Equals, 1)
	waitAndAssert(c, 10*time.Second, d2.checkActiveContainerCount, checker.Equals, 1)
	waitAndAssert(c, 10*time.Second, d3.checkActiveContainerCount, checker.Equals, 1)

	d4 := s.AddDaemon(c, true, false)
	d5 := s.AddDaemon(c, true, false)

	waitAndAssert(c, 10*time.Second, d4.checkActiveContainerCount, checker.Equals, 1)
	waitAndAssert(c, 10*time.Second, d5.checkActiveContainerCount, checker.Equals, 1)
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

	waitAndAssert(c, 20*time.Second, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d2.checkActiveContainerCount, d3.checkActiveContainerCount), checker.Equals, instances)

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

	waitAndAssert(c, 20*time.Second, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d2.checkActiveContainerCount, d3.checkActiveContainerCount), checker.Equals, instances)

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

	waitAndAssert(c, 20*time.Second, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d2.checkActiveContainerCount, d3.checkActiveContainerCount), checker.Equals, instances)

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

	// todo: timeouts do not seem to be implemented yet, this will just hang
	// d1.createService(c, simpleTestService, func(s *swarm.Service) {
	// 	s.Spec.Name = "top2"
	// })
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

	d.updateNode(c, d.getNode(c, nodes[0].ID), func(n *swarm.Node) {
		n.Spec.Availability = swarm.NodeAvailabilityPause
	})

	n := d.getNode(c, nodes[0].ID)
	c.Assert(string(n.Spec.Availability), checker.Equals, swarm.NodeAvailabilityPause)
}

func (s *DockerSwarmSuite) TestApiSwarmNodeDrainPause(c *check.C) {
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, false)

	time.Sleep(1 * time.Second) // make sure all daemons are ready to accept tasks

	// start a service, expect balanced distribution
	instances := 6
	id := d1.createService(c, simpleTestService, setInstances(instances))

	waitAndAssert(c, 10*time.Second, d1.checkActiveContainerCount, checker.GreaterThan, 0)
	waitAndAssert(c, 10*time.Second, d2.checkActiveContainerCount, checker.GreaterThan, 0)
	waitAndAssert(c, 20*time.Second, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d2.checkActiveContainerCount), checker.Equals, instances)

	// drain d2, all containers should move to d1
	d1.updateNode(c, d1.getNode(c, d2.NodeID), func(n *swarm.Node) {
		n.Spec.Availability = swarm.NodeAvailabilityDrain
	})
	waitAndAssert(c, 10*time.Second, d1.checkActiveContainerCount, checker.Equals, 6)
	waitAndAssert(c, 10*time.Second, d2.checkActiveContainerCount, checker.Equals, 0)

	// set d2 back to active
	d1.updateNode(c, d1.getNode(c, d2.NodeID), func(n *swarm.Node) {
		n.Spec.Availability = swarm.NodeAvailabilityActive
	})

	// change environment variable, resulting balanced rescheduling
	d1.updateService(c, d1.getService(c, id), func(s *swarm.Service) {
		s.Spec.TaskSpec.ContainerSpec.Env = []string{"FOO=BAR"}
	})

	waitAndAssert(c, 10*time.Second, d1.checkActiveContainerCount, checker.GreaterThan, 0)
	waitAndAssert(c, 10*time.Second, d2.checkActiveContainerCount, checker.GreaterThan, 0)
	waitAndAssert(c, 20*time.Second, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d2.checkActiveContainerCount), checker.Equals, instances)

	d2ContainerCount := len(d2.activeContainers())

	// set d2 to paused, scale service up, only d1 gets new tasks
	d1.updateNode(c, d1.getNode(c, d2.NodeID), func(n *swarm.Node) {
		n.Spec.Availability = swarm.NodeAvailabilityPause
	})

	instances = 12
	d1.updateService(c, d1.getService(c, id), setInstances(instances))

	waitAndAssert(c, 10*time.Second, d1.checkActiveContainerCount, checker.Equals, instances-d2ContainerCount)
	waitAndAssert(c, 10*time.Second, d2.checkActiveContainerCount, checker.Equals, d2ContainerCount)

}

func (s *DockerSwarmSuite) TestApiSwarmLeaveRemovesContainer(c *check.C) {
	d := s.AddDaemon(c, true, true)

	instances := 2
	d.createService(c, simpleTestService, setInstances(instances))

	id, err := d.Cmd("run", "-d", "busybox", "top")
	c.Assert(err, checker.IsNil)
	id = strings.TrimSpace(id)

	waitAndAssert(c, 10*time.Second, d.checkActiveContainerCount, checker.Equals, instances+1)

	c.Assert(d.Leave(false), checker.NotNil)
	c.Assert(d.Leave(true), checker.IsNil)

	waitAndAssert(c, 10*time.Second, d.checkActiveContainerCount, checker.Equals, 1)

	id2, err := d.Cmd("ps", "-q")
	c.Assert(err, checker.IsNil)
	c.Assert(id, checker.HasPrefix, strings.TrimSpace(id2))
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

func simpleTestService(s *swarm.Service) {
	var uinstances uint64
	uinstances = 1
	s.Spec = swarm.ServiceSpec{
		TaskSpec: swarm.TaskSpec{
			ContainerSpec: swarm.ContainerSpec{
				Image:   "busybox:latest",
				Command: []string{"/bin/top"},
			},
		},
		Mode: swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Instances: &uinstances,
			},
		},
	}
	s.Spec.Name = "top"
}

func setInstances(instances int) serviceConstructor {
	uinstances := uint64(instances)
	return func(s *swarm.Service) {
		s.Spec.Mode = swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Instances: &uinstances,
			},
		}
	}
}

func setGlobalMode(s *swarm.Service) {
	s.Spec.Mode = swarm.ServiceMode{
		Global: &swarm.GlobalService{},
	}
}
