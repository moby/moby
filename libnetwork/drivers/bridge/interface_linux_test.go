package bridge

import (
	"net"
	"net/netip"
	"strings"
	"testing"

	"github.com/docker/docker/internal/testutils/netnsutils"
	"github.com/google/go-cmp/cmp"
	"github.com/vishvananda/netlink"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func cidrToIPNet(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	ip, ipNet, err := net.ParseCIDR(cidr)
	assert.Assert(t, is.Nil(err))
	return &net.IPNet{IP: ip, Mask: ipNet.Mask}
}

func addAddr(t *testing.T, link netlink.Link, addr string) {
	t.Helper()
	ipNet := cidrToIPNet(t, addr)
	err := netlink.AddrAdd(link, &netlink.Addr{IPNet: ipNet})
	assert.Assert(t, is.Nil(err))
}

func prepTestBridge(t *testing.T, nc *networkConfiguration) *bridgeInterface {
	t.Helper()
	nh, err := netlink.NewHandle()
	assert.Assert(t, err)
	i, err := newInterface(nh, nc)
	assert.Assert(t, err)
	err = setupDevice(nc, i)
	assert.Assert(t, err)
	return i
}

func TestInterfaceDefaultName(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	nh, err := netlink.NewHandle()
	if err != nil {
		t.Fatal(err)
	}
	config := &networkConfiguration{}
	_, err = newInterface(nh, config)
	assert.Check(t, err)
	assert.Equal(t, config.BridgeName, DefaultBridgeName)
}

func TestAddressesNoInterface(t *testing.T) {
	i := bridgeInterface{}
	addrs, err := i.addresses(netlink.FAMILY_V6)
	assert.NilError(t, err)
	assert.Check(t, is.Len(addrs, 0))
}

func TestAddressesEmptyInterface(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	nh, err := netlink.NewHandle()
	assert.NilError(t, err)

	inf, err := newInterface(nh, &networkConfiguration{})
	assert.NilError(t, err)

	addrsv4, err := inf.addresses(netlink.FAMILY_V4)
	assert.NilError(t, err)
	assert.Check(t, is.Len(addrsv4, 0))

	addrsv6, err := inf.addresses(netlink.FAMILY_V6)
	assert.NilError(t, err)
	assert.Check(t, is.Len(addrsv6, 0))
}

func TestAddressesNonEmptyInterface(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	i := prepTestBridge(t, &networkConfiguration{})

	const expAddrV4, expAddrV6 = "192.168.1.2/24", "fd00:1234::/64"
	addAddr(t, i.Link, expAddrV4)
	addAddr(t, i.Link, expAddrV6)

	addrs, err := i.addresses(netlink.FAMILY_V4)
	assert.NilError(t, err)
	assert.Check(t, is.Len(addrs, 1))
	assert.Equal(t, addrs[0].IPNet.String(), expAddrV4)

	addrs, err = i.addresses(netlink.FAMILY_V6)
	assert.NilError(t, err)
	assert.Check(t, is.Len(addrs, 1))
	assert.Equal(t, addrs[0].IPNet.String(), expAddrV6)
}

func TestGetRequiredIPv6Addrs(t *testing.T) {
	testcases := []struct {
		name         string
		addressIPv6  string
		expReqdAddrs []string
	}{
		{
			name:         "Regular address, expect default link local",
			addressIPv6:  "2000:3000::1/80",
			expReqdAddrs: []string{"fe80::1/64", "2000:3000::1/80"},
		},
		{
			name:         "Standard link local address only",
			addressIPv6:  "fe80::1/64",
			expReqdAddrs: []string{"fe80::1/64"},
		},
		{
			name:         "Nonstandard link local address",
			addressIPv6:  "fe80:abcd::1/42",
			expReqdAddrs: []string{"fe80:abcd::1/42", "fe80::1/64"},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			config := &networkConfiguration{
				AddressIPv6: cidrToIPNet(t, tc.addressIPv6),
			}

			expResult := map[netip.Addr]netip.Prefix{}
			for _, addr := range tc.expReqdAddrs {
				expResult[netip.MustParseAddr(strings.Split(addr, "/")[0])] = netip.MustParsePrefix(addr)
			}

			reqd, addr, gw, err := getRequiredIPv6Addrs(config)
			assert.Check(t, is.Nil(err))
			assert.Check(t, is.DeepEqual(addr, config.AddressIPv6))
			assert.Check(t, is.DeepEqual(gw, config.AddressIPv6.IP))
			assert.Check(t, is.DeepEqual(reqd, expResult,
				cmp.Comparer(func(a, b netip.Prefix) bool { return a == b })))
		})
	}
}

func TestProgramIPv6Addresses(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	checkAddrs := func(i *bridgeInterface, expAddrs []string) {
		t.Helper()
		exp := []netlink.Addr{}
		for _, a := range expAddrs {
			ipNet := cidrToIPNet(t, a)
			exp = append(exp, netlink.Addr{IPNet: ipNet})
		}
		actual, err := i.addresses(netlink.FAMILY_V6)
		assert.NilError(t, err)
		assert.DeepEqual(t, exp, actual)
	}

	nc := &networkConfiguration{}
	i := prepTestBridge(t, nc)

	// The bridge has no addresses, ask for a regular IPv6 network and expect it to
	// be added to the bridge, with the default link local address.
	nc.AddressIPv6 = cidrToIPNet(t, "2000:3000::1/64")
	err := i.programIPv6Addresses(nc)
	assert.NilError(t, err)
	checkAddrs(i, []string{"2000:3000::1/64", "fe80::1/64"})

	// Shrink the subnet of that regular address, the prefix length of the address
	// will not be modified - but it's informational-only, the address itself has
	// not changed.
	nc.AddressIPv6 = cidrToIPNet(t, "2000:3000::1/80")
	err = i.programIPv6Addresses(nc)
	assert.NilError(t, err)
	checkAddrs(i, []string{"2000:3000::1/64", "fe80::1/64"})

	// Ask for link-local only, by specifying an address with the Link Local prefix.
	// The regular address should be removed.
	nc.AddressIPv6 = cidrToIPNet(t, "fe80::1/64")
	err = i.programIPv6Addresses(nc)
	assert.NilError(t, err)
	checkAddrs(i, []string{"fe80::1/64"})

	// Swap the standard link local address for a nonstandard one.
	nc.AddressIPv6 = cidrToIPNet(t, "fe80:5555::1/55")
	err = i.programIPv6Addresses(nc)
	assert.NilError(t, err)
	checkAddrs(i, []string{"fe80:5555::1/55", "fe80::1/64"})
}
