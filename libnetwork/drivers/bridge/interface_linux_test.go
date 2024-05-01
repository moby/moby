package bridge

import (
	"net"
	"sort"
	"testing"

	"github.com/docker/docker/internal/testutils/netnsutils"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
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

func TestProgramIPv6Addresses(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	checkAddrs := func(i *bridgeInterface, nc *networkConfiguration, expAddrs []string) {
		t.Helper()
		nladdrs, err := i.addresses(netlink.FAMILY_V6)
		actual := []string{}
		for _, a := range nladdrs {
			actual = append(actual, a.String())
		}
		assert.NilError(t, err)
		exp := append([]string(nil), expAddrs...)
		sort.Strings(exp)
		sort.Strings(actual)
		assert.DeepEqual(t, actual, exp)
		assert.Check(t, is.DeepEqual(i.bridgeIPv6, nc.AddressIPv6))
		assert.Check(t, is.DeepEqual(i.gatewayIPv6, nc.AddressIPv6.IP))
	}

	nc := &networkConfiguration{}
	i := prepTestBridge(t, nc)

	// The bridge has no addresses, ask for a regular IPv6 network and expect it to
	// be added to the bridge.
	nc.AddressIPv6 = cidrToIPNet(t, "2000:3000::1/64")
	err := i.programIPv6Addresses(nc)
	assert.NilError(t, err)
	checkAddrs(i, nc, []string{"2000:3000::1/64"})

	// Shrink the subnet of that regular address, the prefix length of the address
	// will not be modified - but it's informational-only, the address itself has
	// not changed.
	nc.AddressIPv6 = cidrToIPNet(t, "2000:3000::1/80")
	err = i.programIPv6Addresses(nc)
	assert.NilError(t, err)
	checkAddrs(i, nc, []string{"2000:3000::1/64"})

	// Ask for link-local only, by specifying an address with the Link Local prefix.
	// The regular address should be removed.
	nc.AddressIPv6 = cidrToIPNet(t, "fe80::1/64")
	err = i.programIPv6Addresses(nc)
	assert.NilError(t, err)
	checkAddrs(i, nc, []string{"fe80::1/64"})

	// Swap the standard link local address for a nonstandard one. The standard LL
	// address will not be removed.
	nc.AddressIPv6 = cidrToIPNet(t, "fe80:5555::1/55")
	err = i.programIPv6Addresses(nc)
	assert.NilError(t, err)
	checkAddrs(i, nc, []string{"fe80:5555::1/55", "fe80::1/64"})

	// Back to the original address, expect the nonstandard LL address to be replaced.
	nc.AddressIPv6 = cidrToIPNet(t, "2000:3000::1/64")
	err = i.programIPv6Addresses(nc)
	assert.NilError(t, err)
	checkAddrs(i, nc, []string{"2000:3000::1/64", "fe80::1/64"})

	// Add a multicast address to the bridge and check it's not removed.
	mcNlAddr, err := netlink.ParseAddr("ff05::db8:0:1234/96")
	assert.NilError(t, err)
	mcNlAddr.Flags = unix.IFA_F_MCAUTOJOIN
	err = i.nlh.AddrAdd(i.Link, mcNlAddr)
	assert.NilError(t, err)
	err = i.programIPv6Addresses(nc)
	assert.NilError(t, err)
	checkAddrs(i, nc, []string{"2000:3000::1/64", "fe80::1/64", "ff05::db8:0:1234/96"})
}
