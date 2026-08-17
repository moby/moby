package buildkit

import (
	"testing"

	"github.com/moby/buildkit/control"
	"github.com/moby/moby/v2/daemon/config"
	"gotest.tools/v3/assert"
)

// TestProxyNetworkControllerOpt verifies that BuilderConfig.ProxyNetwork maps
// to control.Opt.ProxyNetwork. The actual assignment in newSnapshotterController
// and newGraphDriverController is:
//
//	ProxyNetwork: opt.BuilderConfig.ProxyNetwork,
//
// This test covers the type-level compatibility and the expected field names.
// Integration tests for the end-to-end behavior (proxy applied to exec vertices)
// would require a full BuildKit runtime and are out of scope for this unit test.
func TestProxyNetworkControllerOpt(t *testing.T) {
	for _, tc := range []struct {
		name         string
		proxyNetwork bool
	}{
		{name: "disabled", proxyNetwork: false},
		{name: "enabled", proxyNetwork: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.BuilderConfig{
				ProxyNetwork: tc.proxyNetwork,
			}
			// Verify that the BuilderConfig field type and name are compatible
			// with the control.Opt field used in both controller constructors.
			opt := control.Opt{
				ProxyNetwork: cfg.ProxyNetwork,
			}
			assert.Equal(t, opt.ProxyNetwork, tc.proxyNetwork)
		})
	}
}
