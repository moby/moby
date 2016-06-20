package service

import (
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
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
	flags.Set("label", "toadd=newlabel")
	flags.Set("remove-label", "toremove")

	labels := map[string]string{
		"toremove": "thelabeltoremove",
		"tokeep":   "value",
	}

	updateLabels(flags, &labels)
	assert.Equal(t, len(labels), 2)
	assert.Equal(t, labels["tokeep"], "value")
	assert.Equal(t, labels["toadd"], "newlabel")
}

func TestUpdateEnvironment(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("env", "toadd=newenv")
	flags.Set("remove-env", "toremove")

	envs := []string{
		"toremove=theenvtoremove",
		"tokeep=value",
	}

	updateEnvironment(flags, &envs)
	assert.Equal(t, len(envs), 2)
	assert.Equal(t, envs[0], "tokeep=value")
	assert.Equal(t, envs[1], "toadd=newenv")
}

func TestUpdateMounts(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("mount", "type=volume,target=/toadd")
	flags.Set("remove-mount", "/toremove")

	mounts := []swarm.Mount{
		{Target: "/toremove", Type: swarm.MountType("BIND")},
		{Target: "/tokeep", Type: swarm.MountType("BIND")},
	}

	updateMounts(flags, &mounts)
	assert.Equal(t, len(mounts), 2)
	assert.Equal(t, mounts[0].Target, "/tokeep")
	assert.Equal(t, mounts[1].Target, "/toadd")
}

func TestUpdateNetworks(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("network", "toadd")
	flags.Set("remove-network", "toremove")

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
	flags.Set("publish", "1000:1000")
	flags.Set("remove-publish", "333/udp")

	portConfigs := []swarm.PortConfig{
		{TargetPort: 333, Protocol: swarm.PortConfigProtocol("udp")},
		{TargetPort: 555},
	}

	updatePorts(flags, &portConfigs)
	assert.Equal(t, len(portConfigs), 2)
	assert.Equal(t, portConfigs[0].TargetPort, uint32(555))
	assert.Equal(t, portConfigs[1].TargetPort, uint32(1000))
}
