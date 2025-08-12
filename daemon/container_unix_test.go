//go:build linux || freebsd

package daemon

import (
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/config"
	"gotest.tools/v3/assert"
)

// TestContainerWarningHostAndPublishPorts that a warning is returned when setting network mode to host and specifying published ports.
// This should not be tested on Windows because Windows doesn't support "host" network mode.
func TestContainerWarningHostAndPublishPorts(t *testing.T) {
	testCases := []struct {
		ports    map[containertypes.PortProto][]containertypes.PortBinding
		warnings []string
	}{
		{ports: map[containertypes.PortProto][]containertypes.PortBinding{}},
		{ports: map[containertypes.PortProto][]containertypes.PortBinding{
			"8080": {{HostPort: "8989"}},
		}, warnings: []string{"Published ports are discarded when using host network mode"}},
	}
	muteLogs(t)

	for _, tc := range testCases {
		hostConfig := &containertypes.HostConfig{
			Runtime:      "runc",
			NetworkMode:  "host",
			PortBindings: tc.ports,
		}
		d := &Daemon{}
		cfg, err := config.New()
		assert.NilError(t, err)
		rts, err := setupRuntimes(cfg)
		assert.NilError(t, err)
		daemonCfg := &configStore{Config: *cfg, Runtimes: rts}
		wrns, err := d.verifyContainerSettings(daemonCfg, hostConfig, &containertypes.Config{}, false)
		assert.NilError(t, err)
		assert.DeepEqual(t, tc.warnings, wrns)
	}
}
