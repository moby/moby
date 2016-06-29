package service

import (
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/docker/engine-api/types/swarm"
)

func TestUpdateServiceCommandAndArgs(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("command", "the")
	flags.Set("command", "new")
	flags.Set("command", "command")
	flags.Set("arg", "the")
	flags.Set("arg", "new args")

	spec := &swarm.ServiceSpec{}
	cspec := &spec.TaskTemplate.ContainerSpec
	cspec.Command = []string{"old", "command"}
	cspec.Args = []string{"old", "args"}

	updateService(flags, spec)
	assert.EqualStringSlice(t, cspec.Command, []string{"the", "new", "command"})
	assert.EqualStringSlice(t, cspec.Args, []string{"the", "new args"})
}
