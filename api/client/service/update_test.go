package service

import (
	"sort"

	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/swarm"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestUpdateServiceArgs(c *check.C) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("args", "the \"new args\"")

	spec := &swarm.ServiceSpec{}
	cspec := &spec.TaskTemplate.ContainerSpec
	cspec.Args = []string{"old", "args"}

	updateService(flags, spec)
	c.Assert(cspec.Args, check.DeepEquals, []string{"the", "new args"})
}

func (s *DockerSuite) TestUpdateLabels(c *check.C) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("label-add", "toadd=newlabel")
	flags.Set("label-rm", "toremove")

	labels := map[string]string{
		"toremove": "thelabeltoremove",
		"tokeep":   "value",
	}

	updateLabels(flags, &labels)
	c.Assert(len(labels), check.Equals, 2)
	c.Assert(labels["tokeep"], check.Equals, "value")
	c.Assert(labels["toadd"], check.Equals, "newlabel")
}

func (s *DockerSuite) TestUpdateLabelsRemoveALabelThatDoesNotExist(c *check.C) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("label-rm", "dne")

	labels := map[string]string{"foo": "theoldlabel"}
	updateLabels(flags, &labels)
	c.Assert(len(labels), check.Equals, 1)
}

func (s *DockerSuite) TestUpdatePlacement(c *check.C) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("constraint-add", "node=toadd")
	flags.Set("constraint-rm", "node!=toremove")

	placement := &swarm.Placement{
		Constraints: []string{"node!=toremove", "container=tokeep"},
	}

	updatePlacement(flags, placement)
	c.Assert(len(placement.Constraints), check.Equals, 2)
	c.Assert(placement.Constraints[0], check.Equals, "container=tokeep")
	c.Assert(placement.Constraints[1], check.Equals, "node=toadd")
}

func (s *DockerSuite) TestUpdateEnvironment(c *check.C) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("env-add", "toadd=newenv")
	flags.Set("env-rm", "toremove")

	envs := []string{"toremove=theenvtoremove", "tokeep=value"}

	updateEnvironment(flags, &envs)
	c.Assert(len(envs), check.Equals, 2)
	// Order has been removed in updateEnvironment (map)
	sort.Strings(envs)
	c.Assert(envs[0], check.Equals, "toadd=newenv")
	c.Assert(envs[1], check.Equals, "tokeep=value")
}

func (s *DockerSuite) TestUpdateEnvironmentWithDuplicateValues(c *check.C) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("env-add", "foo=newenv")
	flags.Set("env-add", "foo=dupe")
	flags.Set("env-rm", "foo")

	envs := []string{"foo=value"}

	updateEnvironment(flags, &envs)
	c.Assert(len(envs), check.Equals, 0)
}

func (s *DockerSuite) TestUpdateEnvironmentWithDuplicateKeys(c *check.C) {
	// Test case for #25404
	flags := newUpdateCommand(nil).Flags()
	flags.Set("env-add", "A=b")

	envs := []string{"A=c"}

	updateEnvironment(flags, &envs)
	c.Assert(len(envs), check.Equals, 1)
	c.Assert(envs[0], check.Equals, "A=b")
}

func (s *DockerSuite) TestUpdateGroups(c *check.C) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("group-add", "wheel")
	flags.Set("group-add", "docker")
	flags.Set("group-rm", "root")
	flags.Set("group-add", "foo")
	flags.Set("group-rm", "docker")

	groups := []string{"bar", "root"}

	updateGroups(flags, &groups)
	c.Assert(len(groups), check.Equals, 3)
	c.Assert(groups[0], check.Equals, "bar")
	c.Assert(groups[1], check.Equals, "foo")
	c.Assert(groups[2], check.Equals, "wheel")
}

func (s *DockerSuite) TestUpdateMounts(c *check.C) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("mount-add", "type=volume,target=/toadd")
	flags.Set("mount-rm", "/toremove")

	mounts := []mounttypes.Mount{
		{Target: "/toremove", Type: mounttypes.TypeBind},
		{Target: "/tokeep", Type: mounttypes.TypeBind},
	}

	updateMounts(flags, &mounts)
	c.Assert(len(mounts), check.Equals, 2)
	c.Assert(mounts[0].Target, check.Equals, "/tokeep")
	c.Assert(mounts[1].Target, check.Equals, "/toadd")
}

func (s *DockerSuite) TestUpdatePorts(c *check.C) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("publish-add", "1000:1000")
	flags.Set("publish-rm", "333/udp")

	portConfigs := []swarm.PortConfig{
		{TargetPort: 333, Protocol: swarm.PortConfigProtocolUDP},
		{TargetPort: 555},
	}

	err := updatePorts(flags, &portConfigs)
	c.Assert(err, check.IsNil)
	c.Assert(len(portConfigs), check.Equals, 2)
	// Do a sort to have the order (might have changed by map)
	targetPorts := []int{int(portConfigs[0].TargetPort), int(portConfigs[1].TargetPort)}
	sort.Ints(targetPorts)
	c.Assert(targetPorts[0], check.Equals, 555)
	c.Assert(targetPorts[1], check.Equals, 1000)
}

func (s *DockerSuite) TestUpdatePortsDuplicateEntries(c *check.C) {
	// Test case for #25375
	flags := newUpdateCommand(nil).Flags()
	flags.Set("publish-add", "80:80")

	portConfigs := []swarm.PortConfig{
		{TargetPort: 80, PublishedPort: 80},
	}

	err := updatePorts(flags, &portConfigs)
	c.Assert(err, check.IsNil)
	c.Assert(len(portConfigs), check.Equals, 1)
	c.Assert(portConfigs[0].TargetPort, check.Equals, uint32(80))
}

func (s *DockerSuite) TestUpdatePortsDuplicateKeys(c *check.C) {
	// Test case for #25375
	flags := newUpdateCommand(nil).Flags()
	flags.Set("publish-add", "80:20")

	portConfigs := []swarm.PortConfig{
		{TargetPort: 80, PublishedPort: 80},
	}

	err := updatePorts(flags, &portConfigs)
	c.Assert(err, check.IsNil)
	c.Assert(len(portConfigs), check.Equals, 1)
	c.Assert(portConfigs[0].TargetPort, check.Equals, uint32(20))
}

func (s *DockerSuite) TestUpdatePortsConflictingFlags(c *check.C) {
	// Test case for #25375
	flags := newUpdateCommand(nil).Flags()
	flags.Set("publish-add", "80:80")
	flags.Set("publish-add", "80:20")

	portConfigs := []swarm.PortConfig{
		{TargetPort: 80, PublishedPort: 80},
	}

	err := updatePorts(flags, &portConfigs)
	c.Assert(err, check.ErrorMatches, ".*conflicting port mapping.*")
}
