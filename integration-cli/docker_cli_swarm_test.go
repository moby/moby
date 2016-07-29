// +build !windows

package main

import (
	"io/ioutil"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/engine-api/types/swarm"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestSwarmUpdate(c *check.C) {
	d := s.AddDaemon(c, true, true)

	getSpec := func() swarm.Spec {
		sw := d.getSwarm(c)
		return sw.Spec
	}

	out, err := d.Cmd("swarm", "update", "--cert-expiry", "30h", "--dispatcher-heartbeat", "11s")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", out))

	spec := getSpec()
	c.Assert(spec.CAConfig.NodeCertExpiry, checker.Equals, 30*time.Hour)
	c.Assert(spec.Dispatcher.HeartbeatPeriod, checker.Equals, uint64(11*time.Second))

	// setting anything under 30m for cert-expiry is not allowed
	out, err = d.Cmd("swarm", "update", "--cert-expiry", "15m")
	c.Assert(err, checker.NotNil)
	c.Assert(out, checker.Contains, "minimum certificate expiry time")
	spec = getSpec()
	c.Assert(spec.CAConfig.NodeCertExpiry, checker.Equals, 30*time.Hour)
}

func (s *DockerSwarmSuite) TestSwarmInit(c *check.C) {
	d := s.AddDaemon(c, false, false)

	getSpec := func() swarm.Spec {
		sw := d.getSwarm(c)
		return sw.Spec
	}

	out, err := d.Cmd("swarm", "init", "--cert-expiry", "30h", "--dispatcher-heartbeat", "11s")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", out))

	spec := getSpec()
	c.Assert(spec.CAConfig.NodeCertExpiry, checker.Equals, 30*time.Hour)
	c.Assert(spec.Dispatcher.HeartbeatPeriod, checker.Equals, uint64(11*time.Second))

	c.Assert(d.Leave(true), checker.IsNil)

	out, err = d.Cmd("swarm", "init")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", out))

	spec = getSpec()
	c.Assert(spec.CAConfig.NodeCertExpiry, checker.Equals, 90*24*time.Hour)
	c.Assert(spec.Dispatcher.HeartbeatPeriod, checker.Equals, uint64(5*time.Second))
}

func (s *DockerSwarmSuite) TestSwarmInitIPv6(c *check.C) {
	testRequires(c, IPv6)
	d1 := s.AddDaemon(c, false, false)
	out, err := d1.Cmd("swarm", "init", "--listen-addr", "::1")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", out))

	d2 := s.AddDaemon(c, false, false)
	out, err = d2.Cmd("swarm", "join", "::1")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", out))

	out, err = d2.Cmd("info")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", out))
	c.Assert(out, checker.Contains, "Swarm: active")
}

func (s *DockerSwarmSuite) TestSwarmIncompatibleDaemon(c *check.C) {
	// init swarm mode and stop a daemon
	d := s.AddDaemon(c, true, true)
	info, err := d.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)
	c.Assert(d.Stop(), checker.IsNil)

	// start a daemon with --cluster-store and --cluster-advertise
	err = d.Start("--cluster-store=consul://consuladdr:consulport/some/path", "--cluster-advertise=1.1.1.1:2375")
	c.Assert(err, checker.NotNil)
	content, _ := ioutil.ReadFile(d.logFile.Name())
	c.Assert(string(content), checker.Contains, "--cluster-store and --cluster-advertise daemon configurations are incompatible with swarm mode")

	// start a daemon with --live-restore
	err = d.Start("--live-restore")
	c.Assert(err, checker.NotNil)
	content, _ = ioutil.ReadFile(d.logFile.Name())
	c.Assert(string(content), checker.Contains, "--live-restore daemon configuration is incompatible with swarm mode")
	// restart for teardown
	c.Assert(d.Start(), checker.IsNil)
}

// Test case for #24090
func (s *DockerSwarmSuite) TestSwarmNodeListHostname(c *check.C) {
	d := s.AddDaemon(c, true, true)

	// The first line should contain "HOSTNAME"
	out, err := d.Cmd("node", "ls")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.Split(out, "\n")[0], checker.Contains, "HOSTNAME")
}

// Test case for #24270
func (s *DockerSwarmSuite) TestSwarmServiceListFilter(c *check.C) {
	d := s.AddDaemon(c, true, true)

	name1 := "redis-cluster-md5"
	name2 := "redis-cluster"
	name3 := "other-cluster"
	out, err := d.Cmd("service", "create", "--name", name1, "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	out, err = d.Cmd("service", "create", "--name", name2, "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	out, err = d.Cmd("service", "create", "--name", name3, "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	filter1 := "name=redis-cluster-md5"
	filter2 := "name=redis-cluster"

	// We search checker.Contains with `name+" "` to prevent prefix only.
	out, err = d.Cmd("service", "ls", "--filter", filter1)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name1+" ")
	c.Assert(out, checker.Not(checker.Contains), name2+" ")
	c.Assert(out, checker.Not(checker.Contains), name3+" ")

	out, err = d.Cmd("service", "ls", "--filter", filter2)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name1+" ")
	c.Assert(out, checker.Contains, name2+" ")
	c.Assert(out, checker.Not(checker.Contains), name3+" ")

	out, err = d.Cmd("service", "ls")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name1+" ")
	c.Assert(out, checker.Contains, name2+" ")
	c.Assert(out, checker.Contains, name3+" ")
}

func (s *DockerSwarmSuite) TestSwarmServiceTaskList(c *check.C) {
	d := s.AddDaemon(c, true, true)

	name := "service_ps_test"
	out, err := d.Cmd("service", "create", "--replicas=2", "--name", name, "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 2)

	out, err = d.Cmd("service", "ps", name)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name+".1")
	c.Assert(out, checker.Contains, name+".2")

	// Remove one replica to test to make sure ps only shows the correct tasks
	// with and w/o the all flag set.
	out, err = d.Cmd("service", "scale", name+"=1")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 1)

	out, err = d.Cmd("service", "ps", name)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Count, name, 1)
	c.Assert(out, checker.Contains, "Running")
	c.Assert(out, checker.Not(checker.Contains), "Shutdown")

	out, err = d.Cmd("service", "ps", name, "-a")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Count, name, 2)
	c.Assert(out, checker.Contains, "Running")
	c.Assert(out, checker.Contains, "Shutdown")
}

func (s *DockerSwarmSuite) TestSwarmNodeListFilter(c *check.C) {
	d := s.AddDaemon(c, true, true)

	out, err := d.Cmd("node", "inspect", "--format", "{{ .Description.Hostname }}", "self")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")
	name := strings.TrimSpace(out)

	out, err = d.Cmd("node", "inspect", "--format", "{{ .Description.Hostname }}")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Equals, name)

	filter := "name=" + name[:4]

	out, err = d.Cmd("node", "ls", "--filter", filter)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name)

	out, err = d.Cmd("node", "ls", "--filter", "name=none")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Not(checker.Contains), name)
}

func (s *DockerSwarmSuite) TestSwarmNodeTaskListFilter(c *check.C) {
	d := s.AddDaemon(c, true, true)

	name := "redis-cluster-md5"
	out, err := d.Cmd("service", "create", "--name", name, "--replicas=3", "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	// make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 3)

	filter := "name=redis-cluster"

	out, err = d.Cmd("node", "ps", "--filter", filter, "self")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name+".1")
	c.Assert(out, checker.Contains, name+".2")
	c.Assert(out, checker.Contains, name+".3")

	out, err = d.Cmd("node", "ps", "--filter", "name=none", "self")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Not(checker.Contains), name+".1")
	c.Assert(out, checker.Not(checker.Contains), name+".2")
	c.Assert(out, checker.Not(checker.Contains), name+".3")
}

func (s *DockerSwarmSuite) TestSwarmNodeTaskList(c *check.C) {
	d := s.AddDaemon(c, true, true)

	name := "node_ps_test"
	out, err := d.Cmd("service", "create", "--replicas=2", "--name", name, "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 2)

	out, err = d.Cmd("node", "ps")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name+".1")
	c.Assert(out, checker.Contains, name+".2")

	// Remove one replica to test to make sure ps only shows the correct tasks
	// with and w/o the all flag set.
	out, err = d.Cmd("service", "scale", name+"=1")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 1)

	out, err = d.Cmd("node", "ps")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Count, name, 1)
	c.Assert(out, checker.Contains, "Running")

	out, err = d.Cmd("node", "ps", "-f", "desired-state=shutdown")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Count, name, 1)
	c.Assert(out, checker.Contains, "Shutdown")

	out, err = d.Cmd("node", "ps", "-a")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Count, name, 2)
	c.Assert(out, checker.Contains, "Running")
	c.Assert(out, checker.Contains, "Shutdown")

	// All should trump our filter
	out, err = d.Cmd("node", "ps", "-a", "-f", "desired-state=running")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Count, name, 2)
	c.Assert(out, checker.Contains, "Running")
	c.Assert(out, checker.Contains, "Shutdown")
}
