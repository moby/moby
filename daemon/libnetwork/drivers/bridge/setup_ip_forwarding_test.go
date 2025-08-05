//go:build linux

package bridge

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge/internal/firewaller"
	"github.com/moby/moby/v2/internal/testutils/netnsutils"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type ffDropper struct {
	ffDropped4 bool
	ffDropped6 bool
}

func (f *ffDropper) FilterForwardDrop(_ context.Context, ipv firewaller.IPVersion) error {
	if ipv == firewaller.IPv4 {
		f.ffDropped4 = true
	}
	if ipv == firewaller.IPv6 {
		f.ffDropped6 = true
	}
	return nil
}

func TestSetupIPForwarding(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	tests := []struct {
		name    string
		dropper bool
		wantFFD bool
	}{
		{
			// With a firewall modifier (dropper) and config that says filter-forward drop
			// is wanted, expect forwarding to be enabled and a drop policy set.
			dropper: true,
			wantFFD: true,
		},
		{
			// With a firewall modifier that could set the policy to drop, the policy should
			// not be set if not enabled in config.
			dropper: true,
		},
		{
			// With no dropper, forwarding should be enabled. The filter-forward policy
			// cannot be set.
		},
	}
	sysctls := []string{
		ipv4ForwardConf,
		ipv6ForwardConfDefault,
		ipv6ForwardConfAll,
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("dropper=%v/wantFFD=%v", tc.dropper, tc.wantFFD), func(t *testing.T) {
			// Disable IP Forwarding if enabled
			for _, sysctl := range sysctls {
				_, err := configureIPForwarding(sysctl, '0')
				assert.NilError(t, err, "writing %s", sysctl)
			}

			// Set IP Forwarding
			var ffd *ffDropper
			if tc.dropper {
				ffd = &ffDropper{}
			}
			err := setupIPv4Forwarding(ffd, tc.wantFFD)
			assert.NilError(t, err)
			err = setupIPv6Forwarding(ffd, tc.wantFFD)
			assert.NilError(t, err)

			// Check what the firewaller was told.
			if ffd != nil {
				assert.Check(t, is.Equal(ffd.ffDropped4, tc.wantFFD))
				assert.Check(t, is.Equal(ffd.ffDropped6, tc.wantFFD))
			}

			// Read new settings
			for _, sysctl := range sysctls {
				procSetting, err := os.ReadFile(sysctl)
				assert.NilError(t, err, "reading %s", sysctl)
				assert.Check(t, is.Equal(string(procSetting), "1\n"), "checking %s", sysctl)
			}
		})
	}
}
