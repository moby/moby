package container_test

import (
	"testing"

	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
)

func TestValidateContainerState(t *testing.T) {
	tests := []struct {
		state       container.ContainerState
		expectedErr string
	}{
		{state: container.StatePaused},
		{state: container.StateRestarting},
		{state: container.StateRunning},
		{state: container.StateDead},
		{state: container.StateCreated},
		{state: container.StateExited},
		{state: container.StateRemoving},
		{state: "invalid-state-string", expectedErr: `invalid value for state (invalid-state-string): must be one of created, running, paused, restarting, removing, exited, dead`},
	}
	for _, tc := range tests {
		t.Run(string(tc.state), func(t *testing.T) {
			err := container.ValidateContainerState(tc.state)
			if tc.expectedErr == "" {
				assert.NilError(t, err)
			} else {
				assert.Error(t, err, tc.expectedErr)
			}
		})
	}
}
