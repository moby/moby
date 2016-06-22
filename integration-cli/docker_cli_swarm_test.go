// +build !windows

package main

import (
	"encoding/json"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/engine-api/types/swarm"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestSwarmUpdate(c *check.C) {
	d := s.AddDaemon(c, true, true)

	getSpec := func() swarm.Spec {
		out, err := d.Cmd("swarm", "inspect")
		c.Assert(err, checker.IsNil)
		var sw []swarm.Swarm
		c.Assert(json.Unmarshal([]byte(out), &sw), checker.IsNil)
		c.Assert(len(sw), checker.Equals, 1)
		return sw[0].Spec
	}

	out, err := d.Cmd("swarm", "update", "--cert-expiry", "30h", "--dispatcher-heartbeat", "11s", "--auto-accept", "manager", "--auto-accept", "worker", "--secret", "foo")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", out))

	spec := getSpec()
	c.Assert(spec.CAConfig.NodeCertExpiry, checker.Equals, 30*time.Hour)
	c.Assert(spec.Dispatcher.HeartbeatPeriod, checker.Equals, uint64(11*time.Second))

	c.Assert(spec.AcceptancePolicy.Policies, checker.HasLen, 2)

	for _, p := range spec.AcceptancePolicy.Policies {
		c.Assert(p.Autoaccept, checker.Equals, true)
		c.Assert(p.Secret, checker.NotNil)
		c.Assert(*p.Secret, checker.Not(checker.Equals), "")
	}

	out, err = d.Cmd("swarm", "update", "--auto-accept", "none")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", out))

	spec = getSpec()
	c.Assert(spec.CAConfig.NodeCertExpiry, checker.Equals, 30*time.Hour)
	c.Assert(spec.Dispatcher.HeartbeatPeriod, checker.Equals, uint64(11*time.Second))

	c.Assert(spec.AcceptancePolicy.Policies, checker.HasLen, 2)

	for _, p := range spec.AcceptancePolicy.Policies {
		c.Assert(p.Autoaccept, checker.Equals, false)
		// secret is still set
		c.Assert(p.Secret, checker.NotNil)
		c.Assert(*p.Secret, checker.Not(checker.Equals), "")
	}

	out, err = d.Cmd("swarm", "update", "--auto-accept", "manager", "--secret", "")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", out))

	spec = getSpec()

	c.Assert(spec.AcceptancePolicy.Policies, checker.HasLen, 2)

	for _, p := range spec.AcceptancePolicy.Policies {
		c.Assert(p.Autoaccept, checker.Equals, p.Role == swarm.NodeRoleManager)
		// secret has been removed
		c.Assert(p.Secret, checker.IsNil)
	}

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
		out, err := d.Cmd("swarm", "inspect")
		c.Assert(err, checker.IsNil)
		var sw []swarm.Swarm
		c.Assert(json.Unmarshal([]byte(out), &sw), checker.IsNil)
		c.Assert(len(sw), checker.Equals, 1)
		return sw[0].Spec
	}

	out, err := d.Cmd("swarm", "init", "--cert-expiry", "30h", "--dispatcher-heartbeat", "11s", "--auto-accept", "manager", "--auto-accept", "worker", "--secret", "foo")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", out))

	spec := getSpec()
	c.Assert(spec.CAConfig.NodeCertExpiry, checker.Equals, 30*time.Hour)
	c.Assert(spec.Dispatcher.HeartbeatPeriod, checker.Equals, uint64(11*time.Second))

	c.Assert(spec.AcceptancePolicy.Policies, checker.HasLen, 2)

	for _, p := range spec.AcceptancePolicy.Policies {
		c.Assert(p.Autoaccept, checker.Equals, true)
		c.Assert(p.Secret, checker.NotNil)
		c.Assert(*p.Secret, checker.Not(checker.Equals), "")
	}

	c.Assert(d.Leave(true), checker.IsNil)

	out, err = d.Cmd("swarm", "init", "--auto-accept", "none")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", out))

	spec = getSpec()
	c.Assert(spec.CAConfig.NodeCertExpiry, checker.Equals, 90*24*time.Hour)
	c.Assert(spec.Dispatcher.HeartbeatPeriod, checker.Equals, uint64(5*time.Second))

	c.Assert(spec.AcceptancePolicy.Policies, checker.HasLen, 2)

	for _, p := range spec.AcceptancePolicy.Policies {
		c.Assert(p.Autoaccept, checker.Equals, false)
		c.Assert(p.Secret, checker.IsNil)
	}

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
