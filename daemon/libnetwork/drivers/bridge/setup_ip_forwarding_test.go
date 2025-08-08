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
	ffDrop4 bool
	ffDrop6 bool
}

// FilterForwardDrop is part of interface [firewaller.Firewaller]. Just enough to check
// it was called with the expected IPVersion.
func (f *ffDropper) FilterForwardDrop(_ context.Context, ipv firewaller.IPVersion) error {
	switch ipv {
	case firewaller.IPv4:
		f.ffDrop4 = true
	case firewaller.IPv6:
		f.ffDrop6 = true
	}
	return nil
}

func TestSetupIPForwarding(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	for _, wantFFD := range []bool{true, false} {
		t.Run(fmt.Sprintf("wantFFD=%v", wantFFD), func(t *testing.T) {
			// Disable IP Forwarding if enabled
			setForwarding(t, '0')

			// Set IP Forwarding
			ffd := &ffDropper{}
			err := setupIPv4Forwarding(ffd, wantFFD)
			assert.NilError(t, err)

			// Check what the firewaller was told.
			assert.Check(t, is.Equal(ffd.ffDrop4, wantFFD))
			assert.Check(t, !ffd.ffDrop6)

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
			// Disable IP Forwarding if enabled
			setForwarding(t, '0')

			// Set IP Forwarding
			ffd := &ffDropper{}
			err := setupIPv6Forwarding(ffd, wantFFD)
			assert.NilError(t, err)
			assert.Check(t, !ffd.ffDrop4)
			assert.Check(t, is.Equal(ffd.ffDrop6, wantFFD))

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

func TestCheckForwarding(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	setForwarding(t, '0')
	err := checkIPv4Forwarding()
	assert.Check(t, is.ErrorContains(err, "IPv4 forwarding is disabled"))
	err = checkIPv6Forwarding()
	assert.Check(t, is.ErrorContains(err, "IPv6 global forwarding is disabled"))

	setForwarding(t, '1')
	err = checkIPv4Forwarding()
	assert.Check(t, err)
	err = checkIPv6Forwarding()
	assert.Check(t, err)
}

func setForwarding(t *testing.T, val byte) {
	for _, sysctl := range []string{
		ipv4ForwardConf,
		ipv6ForwardConfDefault,
		ipv6ForwardConfAll,
	} {
		err := os.WriteFile(sysctl, []byte{val, '\n'}, 0o644)
		assert.NilError(t, err)
	}
}
