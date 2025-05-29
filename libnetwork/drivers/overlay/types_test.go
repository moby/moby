package overlay

import (
	"net"
	"net/netip"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestMACAddrOf(t *testing.T) {
	want := net.HardwareAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	assert.DeepEqual(t, macAddrOf(want).HardwareAddr(), want)
}

func TestIPMACOf(t *testing.T) {
	assert.Check(t, is.Equal(ipmacOf(netip.Addr{}, nil), ipmac{}))
	assert.Check(t, is.Equal(
		ipmacOf(
			netip.MustParseAddr("11.22.33.44"),
			net.HardwareAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
		),
		ipmac{
			ip:  netip.MustParseAddr("11.22.33.44"),
			mac: macAddrOf(net.HardwareAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}),
		},
	))
}
