package service

import (
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/docker/engine-api/types/swarm"
)

func TestUpdateServiceCommandAndArgs(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("command", "newcommand")
	args := []string{"the", "new args"}

	spec := &swarm.ServiceSpec{}
	cspec := &spec.TaskTemplate.ContainerSpec
	cspec.Command = []string{"old", "command"}
	cspec.Args = []string{"old", "args"}

	updateService(flags, args, spec)
	assert.EqualStringSlice(t, cspec.Command, []string{"newcommand"})
	assert.EqualStringSlice(t, cspec.Args, []string{"the", "new args"})
}

func TestUpdateServiceClearArgs(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	args := []string{""}

	spec := &swarm.ServiceSpec{}
	cspec := &spec.TaskTemplate.ContainerSpec
	cspec.Args = []string{"old", "args"}

	updateService(flags, args, spec)
	assert.EqualStringSlice(t, cspec.Args, []string{})
}

func TestUpdateServiceNoChange(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	args := []string{}

	spec := &swarm.ServiceSpec{}
	cspec := &spec.TaskTemplate.ContainerSpec
	cspec.Args = []string{"old", "args"}

	specCopy := *spec
	cspecCopy := &specCopy.TaskTemplate.ContainerSpec

	updateService(flags, args, spec)
	assert.EqualStringSlice(t, cspec.Args, cspecCopy.Args)
}
