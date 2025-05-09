package container

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestValidateHealthStatus(t *testing.T) {
	tests := []struct {
		health      HealthStatus
		expectedErr string
	}{
		{health: Healthy},
		{health: Unhealthy},
		{health: Starting},
		{health: NoHealthcheck},
		{health: "invalid-health-string", expectedErr: `invalid value for health (invalid-health-string): must be one of none, starting, healthy, unhealthy`},
	}

	for _, tc := range tests {
		t.Run(tc.health, func(t *testing.T) {
			err := ValidateHealthStatus(tc.health)
			if tc.expectedErr == "" {
				assert.NilError(t, err)
			} else {
				assert.Error(t, err, tc.expectedErr)
			}
		})
	}
}
