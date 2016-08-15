package service

import (
	"sort"
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
	mounttypes "github.com/docker/engine-api/types/mount"
	"github.com/docker/engine-api/types/swarm"
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

func TestUpdateNetworks(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("network-add", "toadd")
	flags.Set("network-rm", "toremove")

	attachments := []swarm.NetworkAttachmentConfig{
		{Target: "toremove", Aliases: []string{"foo"}},
		{Target: "tokeep"},
	}

	updateNetworks(flags, &attachments)
	assert.Equal(t, len(attachments), 2)
	assert.Equal(t, attachments[0].Target, "tokeep")
	assert.Equal(t, attachments[1].Target, "toadd")
}

func TestUpdatePorts(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("publish-add", "1000:1000")
	flags.Set("publish-rm", "333/udp")

	portConfigs := []swarm.PortConfig{
		{TargetPort: 333, Protocol: swarm.PortConfigProtocolUDP},
		{TargetPort: 555},
	}

	updatePorts(flags, &portConfigs)
	assert.Equal(t, len(portConfigs), 2)
	assert.Equal(t, portConfigs[0].TargetPort, uint32(555))
	assert.Equal(t, portConfigs[1].TargetPort, uint32(1000))
}
