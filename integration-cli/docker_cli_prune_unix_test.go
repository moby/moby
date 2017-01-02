// +build !windows

package main

import (
	"strconv"
	"strings"

	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/daemon"
	"github.com/go-check/check"
)

func pruneNetworkAndVerify(c *check.C, d *daemon.Swarm, kept, pruned []string) {
	_, err := d.Cmd("network", "prune", "--force")
	c.Assert(err, checker.IsNil)
	out, err := d.Cmd("network", "ls", "--format", "{{.Name}}")
	c.Assert(err, checker.IsNil)
	for _, s := range kept {
		c.Assert(out, checker.Contains, s)
	}
	for _, s := range pruned {
		c.Assert(out, checker.Not(checker.Contains), s)
	}
}

func (s *DockerSwarmSuite) TestPruneNetwork(c *check.C) {
	d := s.AddDaemon(c, true, true)
	_, err := d.Cmd("network", "create", "n1") // used by container (testprune)
	c.Assert(err, checker.IsNil)
	_, err = d.Cmd("network", "create", "n2")
	c.Assert(err, checker.IsNil)
	_, err = d.Cmd("network", "create", "n3", "--driver", "overlay") // used by service (testprunesvc)
	c.Assert(err, checker.IsNil)
	_, err = d.Cmd("network", "create", "n4", "--driver", "overlay")
	c.Assert(err, checker.IsNil)

	cName := "testprune"
	_, err = d.Cmd("run", "-d", "--name", cName, "--net", "n1", "busybox", "top")
	c.Assert(err, checker.IsNil)

	serviceName := "testprunesvc"
	replicas := 1
	out, err := d.Cmd("service", "create", "--name", serviceName,
		"--replicas", strconv.Itoa(replicas),
		"--network", "n3",
		"busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, replicas+1)

	// prune and verify
	pruneNetworkAndVerify(c, d, []string{"n1", "n3"}, []string{"n2", "n4"})

	// remove containers, then prune and verify again
	_, err = d.Cmd("rm", "-f", cName)
	c.Assert(err, checker.IsNil)
	_, err = d.Cmd("service", "rm", serviceName)
	c.Assert(err, checker.IsNil)
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, 0)
	pruneNetworkAndVerify(c, d, []string{}, []string{"n1", "n3"})
}

func (s *DockerDaemonSuite) TestPruneImageDangling(c *check.C) {
	s.d.StartWithBusybox(c)

	out, _, err := s.d.BuildImageWithOut("test",
		`FROM busybox
                 LABEL foo=bar`, true, "-q")
	c.Assert(err, checker.IsNil)
	id := strings.TrimSpace(out)

	out, err = s.d.Cmd("images", "-q", "--no-trunc")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Contains, id)

	out, err = s.d.Cmd("image", "prune", "--force")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Contains), id)

	out, err = s.d.Cmd("images", "-q", "--no-trunc")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Contains, id)

	out, err = s.d.Cmd("image", "prune", "--force", "--all")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Contains, id)

	out, err = s.d.Cmd("images", "-q", "--no-trunc")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Contains), id)
}
