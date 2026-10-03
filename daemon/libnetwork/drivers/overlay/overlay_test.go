//go:build linux

package overlay

import (
	"net/netip"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/pkg/plugingetter"
)

type driverTester struct {
	t *testing.T
	d *driver
}

const testNetworkType = "overlay"

func (dt *driverTester) GetPluginGetter() plugingetter.PluginGetter {
	return nil
}

func (dt *driverTester) RegisterDriver(name string, drv driverapi.Driver, capability driverapi.Capability) error {
	if name != testNetworkType {
		dt.t.Fatalf("Expected driver register name to be %q. Instead got %q",
			testNetworkType, name)
	}

	if _, ok := drv.(*driver); !ok {
		dt.t.Fatalf("Expected driver type to be %T. Instead got %T",
			&driver{}, drv)
	}

	dt.d = drv.(*driver)
	return nil
}

func (dt *driverTester) RegisterNetworkAllocator(name string, _ driverapi.NetworkAllocator) error {
	dt.t.Fatalf("Unexpected call to RegisterNetworkAllocator for %q", name)
	return nil
}

func TestOverlayInit(t *testing.T) {
	if err := Register(&driverTester{t: t}); err != nil {
		t.Fatal(err)
	}
}

func TestOverlayType(t *testing.T) {
	dt := &driverTester{t: t}
	if err := Register(dt); err != nil {
		t.Fatal(err)
	}

	if dt.d.Type() != testNetworkType {
		t.Fatalf("Expected Type() to return %q. Instead got %q", testNetworkType,
			dt.d.Type())
	}
}

func TestMaxMTUTransportFamily(t *testing.T) {
	v4Adv := netip.MustParseAddr("192.0.2.1")
	v6Adv := netip.MustParseAddr("2001:db8::1")
	tests := []struct {
		name     string
		advAddr  netip.Addr
		baseMTU  int
		secure   bool
		expected int
	}{
		{"ipv4 transport", v4Adv, 0, false, 1450},
		{"ipv6 transport", v6Adv, 0, false, 1430},
		{"ipv4 transport custom mtu", v4Adv, 9000, false, 8950},
		{"ipv6 transport custom mtu", v6Adv, 9000, false, 8930},
		{"unknown transport falls back to ipv4", netip.Addr{}, 0, false, 1450},
		{"ipv4 transport encrypted", v4Adv, 0, true, 1424},
		{"ipv6 transport encrypted", v6Adv, 0, true, 1404},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := &driver{advertiseAddress: tc.advAddr}
			n := &network{driver: d, mtu: tc.baseMTU, secure: tc.secure}
			if got := n.maxMTU(); got != tc.expected {
				t.Fatalf("maxMTU() = %d, want %d", got, tc.expected)
			}
		})
	}
}
