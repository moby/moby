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

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

var defaultReconciliationTimeout = 30 * time.Second

func (s *DockerSwarmSuite) TestAPISwarmInit(c *check.C) {
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

func (s *DockerSwarmSuite) TestAPISwarmJoinToken(c *check.C) {
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

func (s *DockerSwarmSuite) TestAPISwarmCAHash(c *check.C) {
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, false, false)
	splitToken := strings.Split(d1.joinTokens(c).Worker, "-")
	splitToken[2] = "1kxftv4ofnc6mt30lmgipg6ngf9luhwqopfk1tz6bdmnkubg0e"
	replacementToken := strings.Join(splitToken, "-")
	err := d2.Join(swarm.JoinRequest{JoinToken: replacementToken, RemoteAddrs: []string{d1.listenAddr}})
	c.Assert(err, checker.NotNil)
	c.Assert(err.Error(), checker.Contains, "remote CA does not match fingerprint")
}

func (s *DockerSwarmSuite) TestAPISwarmPromoteDemote(c *check.C) {
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

func (s *DockerSwarmSuite) TestAPISwarmServicesEmptyList(c *check.C) {
	d := s.AddDaemon(c, true, true)

	services := d.listServices(c)
	c.Assert(services, checker.NotNil)
	c.Assert(len(services), checker.Equals, 0, check.Commentf("services: %#v", services))
}

func (s *DockerSwarmSuite) TestAPISwarmServicesCreate(c *check.C) {
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

func (s *DockerSwarmSuite) TestAPISwarmServicesMultipleAgents(c *check.C) {
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

func (s *DockerSwarmSuite) TestAPISwarmServicesCreateGlobal(c *check.C) {
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

func (s *DockerSwarmSuite) TestAPISwarmServicesUpdate(c *check.C) {
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

func (s *DockerSwarmSuite) TestAPISwarmServiceConstraintRole(c *check.C) {
	const nodeCount = 3
	var daemons [nodeCount]*SwarmDaemon
	for i := 0; i < nodeCount; i++ {
		daemons[i] = s.AddDaemon(c, true, i == 0)
	}
	// wait for nodes ready
	waitAndAssert(c, 5*time.Second, daemons[0].checkNodeReadyCount, checker.Equals, nodeCount)

	// create service
	constraints := []string{"node.role==worker"}
	instances := 3
	id := daemons[0].createService(c, simpleTestService, setConstraints(constraints), setInstances(instances))
	// wait for tasks ready
	waitAndAssert(c, defaultReconciliationTimeout, daemons[0].checkServiceRunningTasks(c, id), checker.Equals, instances)
	// validate tasks are running on worker nodes
	tasks := daemons[0].getServiceTasks(c, id)
	for _, task := range tasks {
		node := daemons[0].getNode(c, task.NodeID)
		c.Assert(node.Spec.Role, checker.Equals, swarm.NodeRoleWorker)
	}
	//remove service
	daemons[0].removeService(c, id)

	// create service
	constraints = []string{"node.role!=worker"}
	id = daemons[0].createService(c, simpleTestService, setConstraints(constraints), setInstances(instances))
	// wait for tasks ready
	waitAndAssert(c, defaultReconciliationTimeout, daemons[0].checkServiceRunningTasks(c, id), checker.Equals, instances)
	tasks = daemons[0].getServiceTasks(c, id)
	// validate tasks are running on manager nodes
	for _, task := range tasks {
		node := daemons[0].getNode(c, task.NodeID)
		c.Assert(node.Spec.Role, checker.Equals, swarm.NodeRoleManager)
	}
	//remove service
	daemons[0].removeService(c, id)

	// create service
	constraints = []string{"node.role==nosuchrole"}
	id = daemons[0].createService(c, simpleTestService, setConstraints(constraints), setInstances(instances))
	// wait for tasks created
	waitAndAssert(c, defaultReconciliationTimeout, daemons[0].checkServiceTasks(c, id), checker.Equals, instances)
	// let scheduler try
	time.Sleep(250 * time.Millisecond)
	// validate tasks are not assigned to any node
	tasks = daemons[0].getServiceTasks(c, id)
	for _, task := range tasks {
		c.Assert(task.NodeID, checker.Equals, "")
	}
}

func (s *DockerSwarmSuite) TestAPISwarmServiceConstraintLabel(c *check.C) {
	const nodeCount = 3
	var daemons [nodeCount]*SwarmDaemon
	for i := 0; i < nodeCount; i++ {
		daemons[i] = s.AddDaemon(c, true, i == 0)
	}
	// wait for nodes ready
	waitAndAssert(c, 5*time.Second, daemons[0].checkNodeReadyCount, checker.Equals, nodeCount)
	nodes := daemons[0].listNodes(c)
	c.Assert(len(nodes), checker.Equals, nodeCount)

	// add labels to nodes
	daemons[0].updateNode(c, nodes[0].ID, func(n *swarm.Node) {
		n.Spec.Annotations.Labels = map[string]string{
			"security": "high",
		}
	})
	for i := 1; i < nodeCount; i++ {
		daemons[0].updateNode(c, nodes[i].ID, func(n *swarm.Node) {
			n.Spec.Annotations.Labels = map[string]string{
				"security": "low",
			}
		})
	}

	// create service
	instances := 3
	constraints := []string{"node.labels.security==high"}
	id := daemons[0].createService(c, simpleTestService, setConstraints(constraints), setInstances(instances))
	// wait for tasks ready
	waitAndAssert(c, defaultReconciliationTimeout, daemons[0].checkServiceRunningTasks(c, id), checker.Equals, instances)
	tasks := daemons[0].getServiceTasks(c, id)
	// validate all tasks are running on nodes[0]
	for _, task := range tasks {
		c.Assert(task.NodeID, checker.Equals, nodes[0].ID)
	}
	//remove service
	daemons[0].removeService(c, id)

	// create service
	constraints = []string{"node.labels.security!=high"}
	id = daemons[0].createService(c, simpleTestService, setConstraints(constraints), setInstances(instances))
	// wait for tasks ready
	waitAndAssert(c, defaultReconciliationTimeout, daemons[0].checkServiceRunningTasks(c, id), checker.Equals, instances)
	tasks = daemons[0].getServiceTasks(c, id)
	// validate all tasks are NOT running on nodes[0]
	for _, task := range tasks {
		c.Assert(task.NodeID, checker.Not(checker.Equals), nodes[0].ID)
	}
	//remove service
	daemons[0].removeService(c, id)

	constraints = []string{"node.labels.security==medium"}
	id = daemons[0].createService(c, simpleTestService, setConstraints(constraints), setInstances(instances))
	// wait for tasks created
	waitAndAssert(c, defaultReconciliationTimeout, daemons[0].checkServiceTasks(c, id), checker.Equals, instances)
	// let scheduler try
	time.Sleep(250 * time.Millisecond)
	tasks = daemons[0].getServiceTasks(c, id)
	// validate tasks are not assigned
	for _, task := range tasks {
		c.Assert(task.NodeID, checker.Equals, "")
	}
	//remove service
	daemons[0].removeService(c, id)

	// multiple constraints
	constraints = []string{
		"node.labels.security==high",
		fmt.Sprintf("node.id==%s", nodes[1].ID),
	}
	id = daemons[0].createService(c, simpleTestService, setConstraints(constraints), setInstances(instances))
	// wait for tasks created
	waitAndAssert(c, defaultReconciliationTimeout, daemons[0].checkServiceTasks(c, id), checker.Equals, instances)
	// let scheduler try
	time.Sleep(250 * time.Millisecond)
	tasks = daemons[0].getServiceTasks(c, id)
	// validate tasks are not assigned
	for _, task := range tasks {
		c.Assert(task.NodeID, checker.Equals, "")
	}
	// make nodes[1] fulfills the constraints
	daemons[0].updateNode(c, nodes[1].ID, func(n *swarm.Node) {
		n.Spec.Annotations.Labels = map[string]string{
			"security": "high",
		}
	})
	// wait for tasks ready
	waitAndAssert(c, defaultReconciliationTimeout, daemons[0].checkServiceRunningTasks(c, id), checker.Equals, instances)
	tasks = daemons[0].getServiceTasks(c, id)
	for _, task := range tasks {
		c.Assert(task.NodeID, checker.Equals, nodes[1].ID)
	}
}

func (s *DockerSwarmSuite) TestAPISwarmServicesStateReporting(c *check.C) {
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

func (s *DockerSwarmSuite) TestAPISwarmLeaderProxy(c *check.C) {
	// add three managers, one of these is leader
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, true)
	d3 := s.AddDaemon(c, true, true)

	// start a service by hitting each of the 3 managers
	d1.createService(c, simpleTestService, func(s *swarm.Service) {
		s.Spec.Name = "test1"
	})
	d2.createService(c, simpleTestService, func(s *swarm.Service) {
		s.Spec.Name = "test2"
	})
	d3.createService(c, simpleTestService, func(s *swarm.Service) {
		s.Spec.Name = "test3"
	})

	// 3 services should be started now, because the requests were proxied to leader
	// query each node and make sure it returns 3 services
	for _, d := range []*SwarmDaemon{d1, d2, d3} {
		services := d.listServices(c)
		c.Assert(services, checker.HasLen, 3)
	}
}

func (s *DockerSwarmSuite) TestAPISwarmLeaderElection(c *check.C) {
	// Create 3 nodes
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, true)
	d3 := s.AddDaemon(c, true, true)

	// assert that the first node we made is the leader, and the other two are followers
	c.Assert(d1.getNode(c, d1.NodeID).ManagerStatus.Leader, checker.True)
	c.Assert(d1.getNode(c, d2.NodeID).ManagerStatus.Leader, checker.False)
	c.Assert(d1.getNode(c, d3.NodeID).ManagerStatus.Leader, checker.False)

	d1.Stop() // stop the leader

	var (
		leader    *SwarmDaemon   // keep track of leader
		followers []*SwarmDaemon // keep track of followers
	)
	checkLeader := func(nodes ...*SwarmDaemon) checkF {
		return func(c *check.C) (interface{}, check.CommentInterface) {
			// clear these out before each run
			leader = nil
			followers = nil
			for _, d := range nodes {
				if d.getNode(c, d.NodeID).ManagerStatus.Leader {
					leader = d
				} else {
					followers = append(followers, d)
				}
			}

			if leader == nil {
				return false, check.Commentf("no leader elected")
			}

			return true, check.Commentf("elected %v", leader.id)
		}
	}

	// wait for an election to occur
	waitAndAssert(c, defaultReconciliationTimeout, checkLeader(d2, d3), checker.True)

	// assert that we have a new leader
	c.Assert(leader, checker.NotNil)

	// Keep track of the current leader, since we want that to be chosen.
	stableleader := leader

	// add the d1, the initial leader, back
	d1.Start()

	// TODO(stevvooe): may need to wait for rejoin here

	// wait for possible election
	waitAndAssert(c, defaultReconciliationTimeout, checkLeader(d1, d2, d3), checker.True)
	// pick out the leader and the followers again

	// verify that we still only have 1 leader and 2 followers
	c.Assert(leader, checker.NotNil)
	c.Assert(followers, checker.HasLen, 2)
	// and that after we added d1 back, the leader hasn't changed
	c.Assert(leader.NodeID, checker.Equals, stableleader.NodeID)
}

func (s *DockerSwarmSuite) TestAPISwarmRaftQuorum(c *check.C) {
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, true)
	d3 := s.AddDaemon(c, true, true)

	d1.createService(c, simpleTestService)

	c.Assert(d2.Stop(), checker.IsNil)

	// make sure there is a leader
	waitAndAssert(c, defaultReconciliationTimeout, d1.checkLeader, checker.IsNil)

	d1.createService(c, simpleTestService, func(s *swarm.Service) {
		s.Spec.Name = "top1"
	})

	c.Assert(d3.Stop(), checker.IsNil)

	// make sure there is a leader
	waitAndAssert(c, defaultReconciliationTimeout, d1.checkLeader, checker.IsNil)

	var service swarm.Service
	simpleTestService(&service)
	service.Spec.Name = "top2"
	status, out, err := d1.SockRequest("POST", "/services/create", service.Spec)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusInternalServerError, check.Commentf("deadline exceeded", string(out)))

	c.Assert(d2.Start(), checker.IsNil)

	// make sure there is a leader
	waitAndAssert(c, defaultReconciliationTimeout, d1.checkLeader, checker.IsNil)

	d1.createService(c, simpleTestService, func(s *swarm.Service) {
		s.Spec.Name = "top3"
	})
}

func (s *DockerSwarmSuite) TestAPISwarmListNodes(c *check.C) {
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

func (s *DockerSwarmSuite) TestAPISwarmNodeUpdate(c *check.C) {
	d := s.AddDaemon(c, true, true)

	nodes := d.listNodes(c)

	d.updateNode(c, nodes[0].ID, func(n *swarm.Node) {
		n.Spec.Availability = swarm.NodeAvailabilityPause
	})

	n := d.getNode(c, nodes[0].ID)
	c.Assert(n.Spec.Availability, checker.Equals, swarm.NodeAvailabilityPause)
}

func (s *DockerSwarmSuite) TestAPISwarmNodeRemove(c *check.C) {
	testRequires(c, Network)
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, false)
	_ = s.AddDaemon(c, true, false)

	nodes := d1.listNodes(c)
	c.Assert(len(nodes), checker.Equals, 3, check.Commentf("nodes: %#v", nodes))

	// Getting the info so we can take the NodeID
	d2Info, err := d2.info()
	c.Assert(err, checker.IsNil)

	// forceful removal of d2 should work
	d1.removeNode(c, d2Info.NodeID, true)

	nodes = d1.listNodes(c)
	c.Assert(len(nodes), checker.Equals, 2, check.Commentf("nodes: %#v", nodes))

	// Restart the node that was removed
	err = d2.Restart()
	c.Assert(err, checker.IsNil)

	// Give some time for the node to rejoin
	time.Sleep(1 * time.Second)

	// Make sure the node didn't rejoin
	nodes = d1.listNodes(c)
	c.Assert(len(nodes), checker.Equals, 2, check.Commentf("nodes: %#v", nodes))
}

func (s *DockerSwarmSuite) TestAPISwarmNodeDrainPause(c *check.C) {
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

func (s *DockerSwarmSuite) TestAPISwarmLeaveRemovesContainer(c *check.C) {
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
func (s *DockerSwarmSuite) TestAPISwarmLeaveOnPendingJoin(c *check.C) {
	testRequires(c, Network)
	s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, false, false)

	id, err := d2.Cmd("run", "-d", "busybox", "top")
	c.Assert(err, checker.IsNil)
	id = strings.TrimSpace(id)

	err = d2.Join(swarm.JoinRequest{
		RemoteAddrs: []string{"123.123.123.123:1234"},
	})
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), checker.Contains, "Timeout was reached")

	info, err := d2.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStatePending)

	c.Assert(d2.Leave(true), checker.IsNil)

	waitAndAssert(c, defaultReconciliationTimeout, d2.checkActiveContainerCount, checker.Equals, 1)

	id2, err := d2.Cmd("ps", "-q")
	c.Assert(err, checker.IsNil)
	c.Assert(id, checker.HasPrefix, strings.TrimSpace(id2))
}

// #23705
func (s *DockerSwarmSuite) TestAPISwarmRestoreOnPendingJoin(c *check.C) {
	testRequires(c, Network)
	d := s.AddDaemon(c, false, false)
	err := d.Join(swarm.JoinRequest{
		RemoteAddrs: []string{"123.123.123.123:1234"},
	})
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), checker.Contains, "Timeout was reached")

	waitAndAssert(c, defaultReconciliationTimeout, d.checkLocalNodeState, checker.Equals, swarm.LocalNodeStatePending)

	c.Assert(d.Stop(), checker.IsNil)
	c.Assert(d.Start(), checker.IsNil)

	info, err := d.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)
}

func (s *DockerSwarmSuite) TestAPISwarmManagerRestore(c *check.C) {
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

func (s *DockerSwarmSuite) TestAPISwarmScaleNoRollingUpdate(c *check.C) {
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

func (s *DockerSwarmSuite) TestAPISwarmInvalidAddress(c *check.C) {
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

func (s *DockerSwarmSuite) TestAPISwarmForceNewCluster(c *check.C) {
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, true)

	instances := 2
	id := d1.createService(c, simpleTestService, setInstances(instances))
	waitAndAssert(c, defaultReconciliationTimeout, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d2.checkActiveContainerCount), checker.Equals, instances)

	// drain d2, all containers should move to d1
	d1.updateNode(c, d2.NodeID, func(n *swarm.Node) {
		n.Spec.Availability = swarm.NodeAvailabilityDrain
	})
	waitAndAssert(c, defaultReconciliationTimeout, d1.checkActiveContainerCount, checker.Equals, instances)
	waitAndAssert(c, defaultReconciliationTimeout, d2.checkActiveContainerCount, checker.Equals, 0)

	c.Assert(d2.Stop(), checker.IsNil)

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
	ureplicas := uint64(1)
	restartDelay := time.Duration(100 * time.Millisecond)

	s.Spec = swarm.ServiceSpec{
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: swarm.ContainerSpec{
				Image:   "busybox:latest",
				Command: []string{"/bin/top"},
			},
			RestartPolicy: &swarm.RestartPolicy{
				Delay: &restartDelay,
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
	ureplicas := uint64(1)
	restartDelay := time.Duration(100 * time.Millisecond)

	s.Spec = swarm.ServiceSpec{
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: swarm.ContainerSpec{
				Image:   "busybox:latest",
				Command: []string{"/bin/top"},
			},
			RestartPolicy: &swarm.RestartPolicy{
				Delay: &restartDelay,
			},
		},
		Mode: swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: &ureplicas,
			},
		},
		UpdateConfig: &swarm.UpdateConfig{
			Parallelism:   2,
			Delay:         4 * time.Second,
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

func setConstraints(constraints []string) serviceConstructor {
	return func(s *swarm.Service) {
		if s.Spec.TaskTemplate.Placement == nil {
			s.Spec.TaskTemplate.Placement = &swarm.Placement{}
		}
		s.Spec.TaskTemplate.Placement.Constraints = constraints
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
		var (
			info swarm.Info
			err  error
		)

		// check info in a waitAndAssert, because if the cluster doesn't have a leader, `info` will return an error
		checkInfo := func(c *check.C) (interface{}, check.CommentInterface) {
			info, err = d.info()
			return err, check.Commentf("cluster not ready in time")
		}
		waitAndAssert(c, defaultReconciliationTimeout, checkInfo, checker.IsNil)
		if !info.ControlAvailable {
			totalWCount++
			continue
		}

		var leaderFound bool
		totalMCount++
		var mCount, wCount int

		for _, n := range d.listNodes(c) {
			waitReady := func(c *check.C) (interface{}, check.CommentInterface) {
				if n.Status.State == swarm.NodeStateReady {
					return true, nil
				}
				nn := d.getNode(c, n.ID)
				n = *nn
				return n.Status.State == swarm.NodeStateReady, check.Commentf("state of node %s, reported by %s", n.ID, d.Info.NodeID)
			}
			waitAndAssert(c, defaultReconciliationTimeout, waitReady, checker.True)

			waitActive := func(c *check.C) (interface{}, check.CommentInterface) {
				if n.Spec.Availability == swarm.NodeAvailabilityActive {
					return true, nil
				}
				nn := d.getNode(c, n.ID)
				n = *nn
				return n.Spec.Availability == swarm.NodeAvailabilityActive, check.Commentf("availability of node %s, reported by %s", n.ID, d.Info.NodeID)
			}
			waitAndAssert(c, defaultReconciliationTimeout, waitActive, checker.True)

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

func (s *DockerSwarmSuite) TestAPISwarmRestartCluster(c *check.C) {
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

func (s *DockerSwarmSuite) TestAPISwarmServicesUpdateWithName(c *check.C) {
	d := s.AddDaemon(c, true, true)

	instances := 2
	id := d.createService(c, simpleTestService, setInstances(instances))
	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, instances)

	service := d.getService(c, id)
	instances = 5

	setInstances(instances)(service)
	url := fmt.Sprintf("/services/%s/update?version=%d", service.Spec.Name, service.Version.Index)
	status, out, err := d.SockRequest("POST", url, service.Spec)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, instances)
}
