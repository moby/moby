package daemon

import (
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/container"
	"gotest.tools/v3/assert"
)

func TestShutdownTimeoutUsesContainerTimeoutCap(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		daemonTimeout      int
		containerTimeout   int
		maxShutdownTimeout int
		expected           int
	}{
		{
			name:             "container timeout without cap",
			containerTimeout: 10,
			expected:         15,
		},
		{
			name:               "container timeout capped",
			containerTimeout:   10,
			maxShutdownTimeout: 5,
			expected:           10,
		},
		{
			name:               "shorter container timeout preserved",
			containerTimeout:   2,
			maxShutdownTimeout: 5,
			expected:           7,
		},
		{
			name:               "indefinite container timeout capped",
			containerTimeout:   -1,
			maxShutdownTimeout: 5,
			expected:           10,
		},
		{
			name:             "indefinite container timeout without cap",
			containerTimeout: -1,
			expected:         -1,
		},
		{
			name:               "cap prevents extending daemon timeout",
			daemonTimeout:      15,
			containerTimeout:   20,
			maxShutdownTimeout: 5,
			expected:           15,
		},
		{
			name:               "negative daemon timeout disables outer deadline",
			daemonTimeout:      -1,
			containerTimeout:   10,
			maxShutdownTimeout: 5,
			expected:           -1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stopTimeout := tc.containerTimeout
			ctr := &container.Container{
				ID: "test",
				Config: &containertypes.Config{
					StopTimeout: &stopTimeout,
				},
			}
			store := container.NewMemoryStore()
			store.Add(ctr.ID, ctr)

			cfg := config.Config{
				CommonConfig: config.CommonConfig{
					ShutdownTimeout:    tc.daemonTimeout,
					MaxShutdownTimeout: tc.maxShutdownTimeout,
				},
			}
			daemon := &Daemon{containers: store}
			daemon.configStore.Store(&configStore{Config: cfg})

			assert.Equal(t, daemon.ShutdownTimeout(), tc.expected)
		})
	}
}
