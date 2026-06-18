//go:build linux || freebsd

package daemon

import (
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"gotest.tools/v3/assert"
)

// TestContainerWarningHostAndPublishPorts that a warning is returned when setting network mode to host and specifying published ports.
// This should not be tested on Windows because Windows doesn't support "host" network mode.
func TestContainerWarningHostAndPublishPorts(t *testing.T) {
	testCases := []struct {
		doc        string
		hostConfig *containertypes.HostConfig
		warnings   []string
	}{
		{
			doc: "ports mapped",
			hostConfig: &containertypes.HostConfig{
				Runtime: "io.containerd.example.v2",
				PortBindings: network.PortMap{
					network.MustParsePort("8080"): []network.PortBinding{{HostPort: "8989"}},
				},
			},
		},
		{
			doc: "host-mode networking without ports mapped",
			hostConfig: &containertypes.HostConfig{
				Runtime:      "io.containerd.example.v2",
				NetworkMode:  "host",
				PortBindings: network.PortMap{},
			},
		},
		{
			doc: "host-mode networking with ports mapped",
			hostConfig: &containertypes.HostConfig{
				Runtime:     "io.containerd.example.v2",
				NetworkMode: "host",
				PortBindings: network.PortMap{
					network.MustParsePort("8080"): []network.PortBinding{{HostPort: "8989"}},
				},
			},
			warnings: []string{"Published ports are discarded when using host network mode"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.doc, func(t *testing.T) {
			d := &Daemon{}
			wrns, err := d.verifyContainerSettings(&configStore{}, tc.hostConfig, &containertypes.Config{}, false)
			assert.NilError(t, err)
			assert.DeepEqual(t, tc.warnings, wrns)
		})
	}
}
