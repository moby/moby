package hashable

import (
	"net"
	"net/netip"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// Assert that the types are hashable.
var (
	_ map[MACAddr]bool
	_ map[IPMAC]bool
)

func TestMACAddrFrom6(t *testing.T) {
	want := [6]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	assert.DeepEqual(t, MACAddrFrom6(want).AsSlice(), want[:])
}

func TestMACAddrFromSlice(t *testing.T) {
	mac, ok := MACAddrFromSlice(net.HardwareAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x06})
	assert.Check(t, ok)
	assert.Check(t, is.DeepEqual(mac.AsSlice(), []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}))

	// Invalid length
	for _, tc := range [][]byte{
		{0x01, 0x02, 0x03, 0x04, 0x05},
		{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
		{},
		nil,
	} {
		mac, ok = MACAddrFromSlice(net.HardwareAddr(tc))
		assert.Check(t, !ok, "want MACAddrFromSlice(%#v) ok=false, got true", tc)
		assert.Check(t, is.DeepEqual(mac.AsSlice(), []byte{0, 0, 0, 0, 0, 0}), "want MACAddrFromSlice(%#v) = %#v, got %#v", tc, []byte{0, 0, 0, 0, 0, 0}, mac.AsSlice())
	}
}

func TestParseMAC(t *testing.T) {
	mac, err := ParseMAC("01:02:03:04:05:06")
	assert.Check(t, err)
	assert.Check(t, is.DeepEqual(mac.AsSlice(), []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}))

	// Invalid MAC address
	_, err = ParseMAC("01:02:03:04:05:06:07:08")
	assert.Check(t, is.ErrorContains(err, "not a MAC-48 address"))
}

func TestMACAddr_String(t *testing.T) {
	mac := MACAddrFrom6([6]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06})
	assert.Check(t, is.Equal(mac.String(), "01:02:03:04:05:06"))
	assert.Check(t, is.Equal(MACAddr(0).String(), "00:00:00:00:00:00"))
}

func TestIPMACFrom(t *testing.T) {
	assert.Check(t, is.Equal(IPMACFrom(netip.Addr{}, 0), IPMAC{}))

	ipm := IPMACFrom(
		netip.MustParseAddr("11.22.33.44"),
		MACAddrFrom6([6]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}),
	)
	assert.Check(t, is.Equal(ipm.IP(), netip.MustParseAddr("11.22.33.44")))
	assert.Check(t, is.Equal(ipm.MAC(), MACAddrFrom6([6]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06})))
}
