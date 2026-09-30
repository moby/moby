package convert

import (
	"testing"

	types "github.com/moby/moby/api/types/swarm"
	swarmapi "github.com/moby/swarmkit/v2/api"
	"gotest.tools/v3/assert"
)

// TestSwarmFromGRPCTaskDefaults tests that TaskDefaults.LogDriver is carried
// over from the swarmkit ClusterSpec to the Engine API Spec. Regression test
// for https://github.com/moby/moby/issues/53396: the field was silently
// dropped because SwarmFromGRPC never read it.
func TestSwarmFromGRPCTaskDefaults(t *testing.T) {
	c := swarmapi.Cluster{
		Spec: swarmapi.ClusterSpec{
			TaskDefaults: swarmapi.TaskDefaults{
				LogDriver: &swarmapi.Driver{
					Name:    "json-file",
					Options: map[string]string{"max-size": "10m", "max-file": "3"},
				},
			},
		},
	}

	swarm := SwarmFromGRPC(c)

	assert.Assert(t, swarm.Spec.TaskDefaults.LogDriver != nil)
	assert.Equal(t, swarm.Spec.TaskDefaults.LogDriver.Name, "json-file")
	assert.DeepEqual(t, swarm.Spec.TaskDefaults.LogDriver.Options, map[string]string{"max-size": "10m", "max-file": "3"})
}

// TestSwarmFromGRPCTaskDefaultsUnset tests that an unset TaskDefaults.LogDriver
// converts to a nil LogDriver rather than an empty, non-nil Driver.
func TestSwarmFromGRPCTaskDefaultsUnset(t *testing.T) {
	c := swarmapi.Cluster{
		Spec: swarmapi.ClusterSpec{},
	}

	swarm := SwarmFromGRPC(c)

	assert.Assert(t, swarm.Spec.TaskDefaults.LogDriver == nil)
}

// TestMergeSwarmSpecToGRPCTaskDefaults tests that TaskDefaults.LogDriver set
// on the Engine API Spec is carried over to the swarmkit ClusterSpec.
// Regression test for https://github.com/moby/moby/issues/53396: the field
// was silently dropped because MergeSwarmSpecToGRPC never wrote it, so
// /swarm/update accepted it and then discarded it.
func TestMergeSwarmSpecToGRPCTaskDefaults(t *testing.T) {
	s := types.Spec{
		TaskDefaults: types.TaskDefaults{
			LogDriver: &types.Driver{
				Name:    "json-file",
				Options: map[string]string{"max-size": "10m", "max-file": "3"},
			},
		},
	}

	spec, err := MergeSwarmSpecToGRPC(s, swarmapi.ClusterSpec{})
	assert.NilError(t, err)

	assert.Assert(t, spec.TaskDefaults.LogDriver != nil)
	assert.Equal(t, spec.TaskDefaults.LogDriver.Name, "json-file")
	assert.DeepEqual(t, spec.TaskDefaults.LogDriver.Options, map[string]string{"max-size": "10m", "max-file": "3"})
}

// TestMergeSwarmSpecToGRPCTaskDefaultsPreservesExisting tests that omitting
// TaskDefaults.LogDriver from an update leaves an already-set value on the
// initial spec untouched, matching how every other optional field in
// MergeSwarmSpecToGRPC behaves (a nil/zero value in the update means "keep
// the status quo", not "clear it").
func TestMergeSwarmSpecToGRPCTaskDefaultsPreservesExisting(t *testing.T) {
	initial := swarmapi.ClusterSpec{
		TaskDefaults: swarmapi.TaskDefaults{
			LogDriver: &swarmapi.Driver{Name: "journald"},
		},
	}

	spec, err := MergeSwarmSpecToGRPC(types.Spec{}, initial)
	assert.NilError(t, err)

	assert.Assert(t, spec.TaskDefaults.LogDriver != nil)
	assert.Equal(t, spec.TaskDefaults.LogDriver.Name, "journald")
}
