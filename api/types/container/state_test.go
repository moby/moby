package container

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestValidateContainerState(t *testing.T) {
	tests := []struct {
		state       ContainerState
		expectedErr string
	}{
		{state: StatePaused},
		{state: StateRestarting},
		{state: StateRunning},
		{state: StateDead},
		{state: StateCreated},
		{state: StateExited},
		{state: StateRemoving},
		{state: "invalid-state-string", expectedErr: `invalid value for state (invalid-state-string): must be one of created, running, paused, restarting, removing, exited, dead`},
	}
	for _, tc := range tests {
		t.Run(tc.state, func(t *testing.T) {
			err := ValidateContainerState(tc.state)
			if tc.expectedErr == "" {
				assert.NilError(t, err)
			} else {
				assert.Error(t, err, tc.expectedErr)
			}
		})
	}
}
