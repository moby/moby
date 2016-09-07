package service

import (
	"sort"

	"github.com/docker/docker/pkg/testutil/assert"
	mounttypes "github.com/docker/engine-api/types/mount"
	"github.com/docker/engine-api/types/swarm"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestUpdateServiceArgs(c *check.C) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("args", "the \"new args\"")

	spec := &swarm.ServiceSpec{}
	cspec := &spec.TaskTemplate.ContainerSpec
	cspec.Args = []string{"old", "args"}

	updateService(flags, spec)
	assert.EqualStringSlice(c, cspec.Args, []string{"the", "new args"})
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
	assert.Equal(c, len(labels), 2)
	assert.Equal(c, labels["tokeep"], "value")
	assert.Equal(c, labels["toadd"], "newlabel")
}

func (s *DockerSuite) TestUpdateLabelsRemoveALabelThatDoesNotExist(c *check.C) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("label-rm", "dne")

	labels := map[string]string{"foo": "theoldlabel"}
	updateLabels(flags, &labels)
	assert.Equal(c, len(labels), 1)
}

func (s *DockerSuite) TestUpdatePlacement(c *check.C) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("constraint-add", "node=toadd")
	flags.Set("constraint-rm", "node!=toremove")

	placement := &swarm.Placement{
		Constraints: []string{"node!=toremove", "container=tokeep"},
	}

	updatePlacement(flags, placement)
	assert.Equal(c, len(placement.Constraints), 2)
	assert.Equal(c, placement.Constraints[0], "container=tokeep")
	assert.Equal(c, placement.Constraints[1], "node=toadd")
}

func (s *DockerSuite) TestUpdateEnvironment(c *check.C) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("env-add", "toadd=newenv")
	flags.Set("env-rm", "toremove")

	envs := []string{"toremove=theenvtoremove", "tokeep=value"}

	updateEnvironment(flags, &envs)
	assert.Equal(c, len(envs), 2)
	// Order has been removed in updateEnvironment (map)
	sort.Strings(envs)
	assert.Equal(c, envs[0], "toadd=newenv")
	assert.Equal(c, envs[1], "tokeep=value")
}

func (s *DockerSuite) TestUpdateEnvironmentWithDuplicateValues(c *check.C) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("env-add", "foo=newenv")
	flags.Set("env-add", "foo=dupe")
	flags.Set("env-rm", "foo")

	envs := []string{"foo=value"}

	updateEnvironment(flags, &envs)
	assert.Equal(c, len(envs), 0)
}

func (s *DockerSuite) TestUpdateEnvironmentWithDuplicateKeys(c *check.C) {
	// Test case for #25404
	flags := newUpdateCommand(nil).Flags()
	flags.Set("env-add", "A=b")

	envs := []string{"A=c"}

	updateEnvironment(flags, &envs)
	assert.Equal(c, len(envs), 1)
	assert.Equal(c, envs[0], "A=b")
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
	assert.Equal(c, len(groups), 3)
	assert.Equal(c, groups[0], "bar")
	assert.Equal(c, groups[1], "foo")
	assert.Equal(c, groups[2], "wheel")
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
	assert.Equal(c, len(mounts), 2)
	assert.Equal(c, mounts[0].Target, "/tokeep")
	assert.Equal(c, mounts[1].Target, "/toadd")
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
	assert.Equal(c, err, nil)
	assert.Equal(c, len(portConfigs), 2)
	// Do a sort to have the order (might have changed by map)
	targetPorts := []int{int(portConfigs[0].TargetPort), int(portConfigs[1].TargetPort)}
	sort.Ints(targetPorts)
	assert.Equal(c, targetPorts[0], 555)
	assert.Equal(c, targetPorts[1], 1000)
}

func (s *DockerSuite) TestUpdatePortsDuplicateEntries(c *check.C) {
	// Test case for #25375
	flags := newUpdateCommand(nil).Flags()
	flags.Set("publish-add", "80:80")

	portConfigs := []swarm.PortConfig{
		{TargetPort: 80, PublishedPort: 80},
	}

	err := updatePorts(flags, &portConfigs)
	assert.Equal(c, err, nil)
	assert.Equal(c, len(portConfigs), 1)
	assert.Equal(c, portConfigs[0].TargetPort, uint32(80))
}

func (s *DockerSuite) TestUpdatePortsDuplicateKeys(c *check.C) {
	// Test case for #25375
	flags := newUpdateCommand(nil).Flags()
	flags.Set("publish-add", "80:20")

	portConfigs := []swarm.PortConfig{
		{TargetPort: 80, PublishedPort: 80},
	}

	err := updatePorts(flags, &portConfigs)
	assert.Equal(c, err, nil)
	assert.Equal(c, len(portConfigs), 1)
	assert.Equal(c, portConfigs[0].TargetPort, uint32(20))
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
	assert.Error(c, err, "conflicting port mapping")
}
