//go:build linux || freebsd
// +build linux freebsd

package daemon

import (
	"testing"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/go-connections/nat"
	"gotest.tools/v3/assert"
)

// TestContainerWarningHostAndPublishPorts that a warning is returned when setting network mode to host and specifying published ports.
// This should not be tested on Windows because Windows doesn't support "host" network mode.
func TestContainerWarningHostAndPublishPorts(t *testing.T) {
	testCases := []struct {
		ports    nat.PortMap
		warnings []string
	}{
		{ports: nat.PortMap{}},
		{ports: nat.PortMap{
			"8080": []nat.PortBinding{{HostPort: "8989"}},
		}, warnings: []string{"Published ports are discarded when using host network mode"}},
	}
	muteLogs()

	for _, tc := range testCases {
		hostConfig := &containertypes.HostConfig{
			Runtime:      "runc",
			NetworkMode:  "host",
			PortBindings: tc.ports,
		}
		cs := &config.Config{
			Runtimes: map[string]types.Runtime{"runc": {}},
		}
		d := &Daemon{configStore: cs}
		wrns, err := d.verifyContainerSettings(hostConfig, &containertypes.Config{}, false)
		assert.NilError(t, err)
		assert.DeepEqual(t, tc.warnings, wrns)
	}
}
