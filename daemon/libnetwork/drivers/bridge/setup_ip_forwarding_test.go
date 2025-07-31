//go:build linux

package bridge

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/docker/docker/daemon/libnetwork/drivers/bridge/internal/firewaller"
	"github.com/docker/docker/internal/testutils/netnsutils"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type ffdTestFirewaller struct {
	ffd firewaller.IPVersion
}

// NewNetwork is part of interface [firewaller.Firewaller].
func (f *ffdTestFirewaller) NewNetwork(_ context.Context, _ firewaller.NetworkConfig) (firewaller.Network, error) {
	return nil, nil
}

// FilterForwardDrop is part of interface [firewaller.Firewaller]. Just enough to check
// it was called with the expected IPVersion.
func (f *ffdTestFirewaller) FilterForwardDrop(_ context.Context, ipv firewaller.IPVersion) error {
	f.ffd = ipv
	return nil
}

func TestSetupIPForwarding(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	for _, wantFFD := range []bool{true, false} {
		t.Run(fmt.Sprintf("wantFFD=%v", wantFFD), func(t *testing.T) {
			fw := &ffdTestFirewaller{}
			d := &driver{
				config: configuration{
					EnableIPForwarding:       true,
					DisableFilterForwardDrop: !wantFFD,
					EnableIPTables:           true,
					EnableIP6Tables:          true,
				},
				firewaller: fw,
			}

			// Disable IP Forwarding if enabled
			_, err := configureIPForwarding(ipv4ForwardConf, '0')
			assert.NilError(t, err)

			// Set IP Forwarding
			err = d.setupIPv4Forwarding(context.Background())
			assert.NilError(t, err)

			// Check what the firewaller was told.
			if wantFFD {
				assert.Check(t, is.Equal(fw.ffd, firewaller.IPv4))
			} else {
				var noVer firewaller.IPVersion
				assert.Check(t, is.Equal(fw.ffd, noVer))
			}

			// Read new setting
			procSetting, err := os.ReadFile(ipv4ForwardConf)
			assert.NilError(t, err)
			assert.Check(t, is.DeepEqual(procSetting, []byte{'1', '\n'}))
		})
	}
}

func TestSetupIP6Forwarding(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	for _, wantFFD := range []bool{true, false} {
		t.Run(fmt.Sprintf("wantFFD=%v", wantFFD), func(t *testing.T) {
			fw := &ffdTestFirewaller{}
			d := &driver{
				config: configuration{
					EnableIPForwarding:       true,
					DisableFilterForwardDrop: !wantFFD,
					EnableIPTables:           true,
					EnableIP6Tables:          true,
				},
				firewaller: fw,
			}

			_, err := configureIPForwarding(ipv6ForwardConfDefault, '0')
			assert.NilError(t, err)
			_, err = configureIPForwarding(ipv6ForwardConfAll, '0')
			assert.NilError(t, err)

			// Set IP Forwarding
			err = d.setupIPv6Forwarding(context.Background())
			assert.NilError(t, err)

			// Check what the firewaller was told.
			if wantFFD {
				assert.Check(t, is.Equal(fw.ffd, firewaller.IPv6))
			} else {
				var noVer firewaller.IPVersion
				assert.Check(t, is.Equal(fw.ffd, noVer))
			}

			// Read new setting
			procSetting, err := os.ReadFile(ipv6ForwardConfDefault)
			assert.NilError(t, err)
			assert.Check(t, is.DeepEqual(procSetting, []byte{'1', '\n'}))
			procSetting, err = os.ReadFile(ipv6ForwardConfAll)
			assert.NilError(t, err)
			assert.Check(t, is.DeepEqual(procSetting, []byte{'1', '\n'}))
		})
	}
}
