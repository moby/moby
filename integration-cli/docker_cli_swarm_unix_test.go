// +build !windows

package main

import (
	"encoding/json"
	"strings"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestSwarmVolumePlugin(c *check.C) {
	d := s.AddDaemon(c, true, true)

	out, err := d.Cmd("service", "create", "--mount", "type=volume,source=my-volume,destination=/foo,volume-driver=customvolumedriver", "--name", "top", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	// Make sure task stays pending before plugin is available
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckServiceTasksInState("top", swarm.TaskStatePending, "missing plugin on 1 node"), checker.Equals, 1)

	plugin := newVolumePlugin(c, "customvolumedriver")
	defer plugin.Close()

	// create a dummy volume to trigger lazy loading of the plugin
	out, err = d.Cmd("volume", "create", "-d", "customvolumedriver", "hello")

	// TODO(aaronl): It will take about 15 seconds for swarm to realize the
	// plugin was loaded. Switching the test over to plugin v2 would avoid
	// this long delay.

	// make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, 1)

	out, err = d.Cmd("ps", "-q")
	c.Assert(err, checker.IsNil)
	containerID := strings.TrimSpace(out)

	out, err = d.Cmd("inspect", "-f", "{{json .Mounts}}", containerID)
	c.Assert(err, checker.IsNil)

	var mounts []struct {
		Name   string
		Driver string
	}

	c.Assert(json.NewDecoder(strings.NewReader(out)).Decode(&mounts), checker.IsNil)
	c.Assert(len(mounts), checker.Equals, 1, check.Commentf(out))
	c.Assert(mounts[0].Name, checker.Equals, "my-volume")
	c.Assert(mounts[0].Driver, checker.Equals, "customvolumedriver")
}
