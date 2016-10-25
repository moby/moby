package service

import (
	"sort"
	"testing"

	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestUpdateServiceArgs(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("args", "the \"new args\"")

	spec := &swarm.ServiceSpec{}
	cspec := &spec.TaskTemplate.ContainerSpec
	cspec.Args = []string{"old", "args"}

	updateService(flags, spec)
	assert.EqualStringSlice(t, cspec.Args, []string{"the", "new args"})
}

func TestUpdateLabels(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("label-add", "toadd=newlabel")
	flags.Set("label-rm", "toremove")

	labels := map[string]string{
		"toremove": "thelabeltoremove",
		"tokeep":   "value",
	}

	updateLabels(flags, &labels)
	assert.Equal(t, len(labels), 2)
	assert.Equal(t, labels["tokeep"], "value")
	assert.Equal(t, labels["toadd"], "newlabel")
}

func TestUpdateLabelsRemoveALabelThatDoesNotExist(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("label-rm", "dne")

	labels := map[string]string{"foo": "theoldlabel"}
	updateLabels(flags, &labels)
	assert.Equal(t, len(labels), 1)
}

func TestUpdatePlacement(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("constraint-add", "node=toadd")
	flags.Set("constraint-rm", "node!=toremove")

	placement := &swarm.Placement{
		Constraints: []string{"node!=toremove", "container=tokeep"},
	}

	updatePlacement(flags, placement)
	assert.Equal(t, len(placement.Constraints), 2)
	assert.Equal(t, placement.Constraints[0], "container=tokeep")
	assert.Equal(t, placement.Constraints[1], "node=toadd")
}

func TestUpdateEnvironment(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("env-add", "toadd=newenv")
	flags.Set("env-rm", "toremove")

	envs := []string{"toremove=theenvtoremove", "tokeep=value"}

	updateEnvironment(flags, &envs)
	assert.Equal(t, len(envs), 2)
	// Order has been removed in updateEnvironment (map)
	sort.Strings(envs)
	assert.Equal(t, envs[0], "toadd=newenv")
	assert.Equal(t, envs[1], "tokeep=value")
}

func TestUpdateEnvironmentWithDuplicateValues(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("env-add", "foo=newenv")
	flags.Set("env-add", "foo=dupe")
	flags.Set("env-rm", "foo")

	envs := []string{"foo=value"}

	updateEnvironment(flags, &envs)
	assert.Equal(t, len(envs), 0)
}

func TestUpdateEnvironmentWithDuplicateKeys(t *testing.T) {
	// Test case for #25404
	flags := newUpdateCommand(nil).Flags()
	flags.Set("env-add", "A=b")

	envs := []string{"A=c"}

	updateEnvironment(flags, &envs)
	assert.Equal(t, len(envs), 1)
	assert.Equal(t, envs[0], "A=b")
}

func TestUpdateGroups(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("group-add", "wheel")
	flags.Set("group-add", "docker")
	flags.Set("group-rm", "root")
	flags.Set("group-add", "foo")
	flags.Set("group-rm", "docker")

	groups := []string{"bar", "root"}

	updateGroups(flags, &groups)
	assert.Equal(t, len(groups), 3)
	assert.Equal(t, groups[0], "bar")
	assert.Equal(t, groups[1], "foo")
	assert.Equal(t, groups[2], "wheel")
}

func TestUpdateMounts(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("mount-add", "type=volume,target=/toadd")
	flags.Set("mount-rm", "/toremove")

	mounts := []mounttypes.Mount{
		{Target: "/toremove", Type: mounttypes.TypeBind},
		{Target: "/tokeep", Type: mounttypes.TypeBind},
	}

	updateMounts(flags, &mounts)
	assert.Equal(t, len(mounts), 2)
	assert.Equal(t, mounts[0].Target, "/tokeep")
	assert.Equal(t, mounts[1].Target, "/toadd")
}

func TestUpdatePorts(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("publish-add", "1000:1000")
	flags.Set("publish-rm", "333/udp")

	portConfigs := []swarm.PortConfig{
		{TargetPort: 333, Protocol: swarm.PortConfigProtocolUDP},
		{TargetPort: 555},
	}

	err := updatePorts(flags, &portConfigs)
	assert.Equal(t, err, nil)
	assert.Equal(t, len(portConfigs), 2)
	// Do a sort to have the order (might have changed by map)
	targetPorts := []int{int(portConfigs[0].TargetPort), int(portConfigs[1].TargetPort)}
	sort.Ints(targetPorts)
	assert.Equal(t, targetPorts[0], 555)
	assert.Equal(t, targetPorts[1], 1000)
}

func TestUpdatePortsDuplicateEntries(t *testing.T) {
	// Test case for #25375
	flags := newUpdateCommand(nil).Flags()
	flags.Set("publish-add", "80:80")

	portConfigs := []swarm.PortConfig{
		{TargetPort: 80, PublishedPort: 80},
	}

	err := updatePorts(flags, &portConfigs)
	assert.Equal(t, err, nil)
	assert.Equal(t, len(portConfigs), 1)
	assert.Equal(t, portConfigs[0].TargetPort, uint32(80))
}

func TestUpdatePortsDuplicateKeys(t *testing.T) {
	// Test case for #25375
	flags := newUpdateCommand(nil).Flags()
	flags.Set("publish-add", "80:20")

	portConfigs := []swarm.PortConfig{
		{TargetPort: 80, PublishedPort: 80},
	}

	err := updatePorts(flags, &portConfigs)
	assert.Equal(t, err, nil)
	assert.Equal(t, len(portConfigs), 1)
	assert.Equal(t, portConfigs[0].TargetPort, uint32(20))
}

func TestUpdatePortsConflictingFlags(t *testing.T) {
	// Test case for #25375
	flags := newUpdateCommand(nil).Flags()
	flags.Set("publish-add", "80:80")
	flags.Set("publish-add", "80:20")

	portConfigs := []swarm.PortConfig{
		{TargetPort: 80, PublishedPort: 80},
	}

	err := updatePorts(flags, &portConfigs)
	assert.Error(t, err, "conflicting port mapping")
}
