package container_test

import (
	"testing"

	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
)

func TestValidateHealthStatus(t *testing.T) {
	tests := []struct {
		health      container.HealthStatus
		expectedErr string
	}{
		{health: container.Healthy},
		{health: container.Unhealthy},
		{health: container.Starting},
		{health: container.NoHealthcheck},
		{health: "invalid-health-string", expectedErr: `invalid value for health (invalid-health-string): must be one of none, starting, healthy, unhealthy`},
	}

	for _, tc := range tests {
		t.Run(string(tc.health), func(t *testing.T) {
			err := container.ValidateHealthStatus(tc.health)
			if tc.expectedErr == "" {
				assert.NilError(t, err)
			} else {
				assert.Error(t, err, tc.expectedErr)
			}
		})
	}
}
