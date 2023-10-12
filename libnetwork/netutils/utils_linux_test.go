package netutils

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/docker/docker/internal/testutils/netnsutils"
	"github.com/docker/docker/libnetwork/ipamutils"
	"github.com/docker/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestNonOverlappingNameservers(t *testing.T) {
	network := &net.IPNet{
		IP:   []byte{192, 168, 0, 1},
		Mask: []byte{255, 255, 255, 0},
	}
	nameservers := []string{
		"127.0.0.1/32",
	}

	if err := CheckNameserverOverlaps(nameservers, network); err != nil {
		t.Fatal(err)
	}
}

func TestOverlappingNameservers(t *testing.T) {
	network := &net.IPNet{
		IP:   []byte{192, 168, 0, 1},
		Mask: []byte{255, 255, 255, 0},
	}
	nameservers := []string{
		"192.168.0.1/32",
	}

	if err := CheckNameserverOverlaps(nameservers, network); err == nil {
		t.Fatalf("Expected error %s got %s", ErrNetworkOverlapsWithNameservers, err)
	}
}

func TestCheckRouteOverlaps(t *testing.T) {
	networkGetRoutesFct = func(netlink.Link, int) ([]netlink.Route, error) {
		routesData := []string{"10.0.2.0/32", "10.0.3.0/24", "10.0.42.0/24", "172.16.42.0/24", "192.168.142.0/24"}
		routes := []netlink.Route{}
		for _, addr := range routesData {
			_, netX, _ := net.ParseCIDR(addr)
			routes = append(routes, netlink.Route{Dst: netX})
		}
		return routes, nil
	}
	defer func() { networkGetRoutesFct = nil }()

	_, netX, _ := net.ParseCIDR("172.16.0.1/24")
	if err := CheckRouteOverlaps(netX); err != nil {
		t.Fatal(err)
	}

	_, netX, _ = net.ParseCIDR("10.0.2.0/24")
	if err := CheckRouteOverlaps(netX); err == nil {
		t.Fatal("10.0.2.0/24 and 10.0.2.0 should overlap but it doesn't")
	}
}

func TestCheckNameserverOverlaps(t *testing.T) {
	nameservers := []string{"10.0.2.3/32", "192.168.102.1/32"}

	_, netX, _ := net.ParseCIDR("10.0.2.3/32")

	if err := CheckNameserverOverlaps(nameservers, netX); err == nil {
		t.Fatalf("%s should overlap 10.0.2.3/32 but doesn't", netX)
	}

	_, netX, _ = net.ParseCIDR("192.168.102.2/32")

	if err := CheckNameserverOverlaps(nameservers, netX); err != nil {
		t.Fatalf("%s should not overlap %v but it does", netX, nameservers)
	}
}

func AssertOverlap(CIDRx string, CIDRy string, t *testing.T) {
	_, netX, _ := net.ParseCIDR(CIDRx)
	_, netY, _ := net.ParseCIDR(CIDRy)
	if !NetworkOverlaps(netX, netY) {
		t.Errorf("%v and %v should overlap", netX, netY)
	}
}

func AssertNoOverlap(CIDRx string, CIDRy string, t *testing.T) {
	_, netX, _ := net.ParseCIDR(CIDRx)
	_, netY, _ := net.ParseCIDR(CIDRy)
	if NetworkOverlaps(netX, netY) {
		t.Errorf("%v and %v should not overlap", netX, netY)
	}
}

func TestNetworkOverlaps(t *testing.T) {
	// netY starts at same IP and ends within netX
	AssertOverlap("172.16.0.1/24", "172.16.0.1/25", t)
	// netY starts within netX and ends at same IP
	AssertOverlap("172.16.0.1/24", "172.16.0.128/25", t)
	// netY starts and ends within netX
	AssertOverlap("172.16.0.1/24", "172.16.0.64/25", t)
	// netY starts at same IP and ends outside of netX
	AssertOverlap("172.16.0.1/24", "172.16.0.1/23", t)
	// netY starts before and ends at same IP of netX
	AssertOverlap("172.16.1.1/24", "172.16.0.1/23", t)
	// netY starts before and ends outside of netX
	AssertOverlap("172.16.1.1/24", "172.16.0.1/22", t)
	// netY starts and ends before netX
	AssertNoOverlap("172.16.1.1/25", "172.16.0.1/24", t)
	// netX starts and ends before netY
	AssertNoOverlap("172.16.1.1/25", "172.16.2.1/24", t)
}

func TestNetworkRange(t *testing.T) {
	// Simple class C test
	_, network, _ := net.ParseCIDR("192.168.0.1/24")
	first, last := NetworkRange(network)
	if !first.Equal(net.ParseIP("192.168.0.0")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("192.168.0.255")) {
		t.Error(last.String())
	}

	// Class A test
	_, network, _ = net.ParseCIDR("10.0.0.1/8")
	first, last = NetworkRange(network)
	if !first.Equal(net.ParseIP("10.0.0.0")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("10.255.255.255")) {
		t.Error(last.String())
	}

	// Class A, random IP address
	_, network, _ = net.ParseCIDR("10.1.2.3/8")
	first, last = NetworkRange(network)
	if !first.Equal(net.ParseIP("10.0.0.0")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("10.255.255.255")) {
		t.Error(last.String())
	}

	// 32bit mask
	_, network, _ = net.ParseCIDR("10.1.2.3/32")
	first, last = NetworkRange(network)
	if !first.Equal(net.ParseIP("10.1.2.3")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("10.1.2.3")) {
		t.Error(last.String())
	}

	// 31bit mask
	_, network, _ = net.ParseCIDR("10.1.2.3/31")
	first, last = NetworkRange(network)
	if !first.Equal(net.ParseIP("10.1.2.2")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("10.1.2.3")) {
		t.Error(last.String())
	}

	// 26bit mask
	_, network, _ = net.ParseCIDR("10.1.2.3/26")
	first, last = NetworkRange(network)
	if !first.Equal(net.ParseIP("10.1.2.0")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("10.1.2.63")) {
		t.Error(last.String())
	}
}

// Test veth name generation "veth"+rand (e.g.veth0f60e2c)
func TestGenerateRandomName(t *testing.T) {
	const vethPrefix = "veth"
	const vethLen = len(vethPrefix) + 7

	testCases := []struct {
		prefix string
		length int
		error  bool
	}{
		{vethPrefix, -1, true},
		{vethPrefix, 0, true},
		{vethPrefix, len(vethPrefix) - 1, true},
		{vethPrefix, len(vethPrefix), true},
		{vethPrefix, len(vethPrefix) + 1, false},
		{vethPrefix, 255, false},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("prefix=%s/length=%d", tc.prefix, tc.length), func(t *testing.T) {
			name, err := GenerateRandomName(tc.prefix, tc.length)
			if tc.error {
				assert.Check(t, is.ErrorContains(err, "invalid length"))
			} else {
				assert.NilError(t, err)
				assert.Check(t, strings.HasPrefix(name, tc.prefix), "Expected name to start with %s", tc.prefix)
				assert.Check(t, is.Equal(len(name), tc.length), "Expected %d characters, instead received %d characters", tc.length, len(name))
			}
		})
	}

	var randomNames [16]string
	for i := range randomNames {
		randomName, err := GenerateRandomName(vethPrefix, vethLen)
		assert.NilError(t, err)

		for _, oldName := range randomNames {
			if randomName == oldName {
				t.Fatalf("Duplicate random name generated: %s", randomName)
			}
		}

		randomNames[i] = randomName
	}
}

// Test mac generation.
func TestUtilGenerateRandomMAC(t *testing.T) {
	mac1 := GenerateRandomMAC()
	mac2 := GenerateRandomMAC()
	// ensure bytes are unique
	if bytes.Equal(mac1, mac2) {
		t.Fatalf("mac1 %s should not equal mac2 %s", mac1, mac2)
	}
	// existing tests check string functionality so keeping the pattern
	if mac1.String() == mac2.String() {
		t.Fatalf("mac1 %s should not equal mac2 %s", mac1, mac2)
	}
}

func TestNetworkRequest(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	nw, err := FindAvailableNetwork(ipamutils.GetLocalScopeDefaultNetworks())
	if err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, exp := range ipamutils.GetLocalScopeDefaultNetworks() {
		if types.CompareIPNet(exp, nw) {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Found unexpected broad network %s", nw)
	}

	nw, err = FindAvailableNetwork(ipamutils.GetGlobalScopeDefaultNetworks())
	if err != nil {
		t.Fatal(err)
	}

	found = false
	for _, exp := range ipamutils.GetGlobalScopeDefaultNetworks() {
		if types.CompareIPNet(exp, nw) {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Found unexpected granular network %s", nw)
	}

	// Add iface and ssert returned address on request
	createInterface(t, "test", "172.17.42.1/16")

	_, exp, err := net.ParseCIDR("172.18.0.0/16")
	if err != nil {
		t.Fatal(err)
	}
	nw, err = FindAvailableNetwork(ipamutils.GetLocalScopeDefaultNetworks())
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(exp, nw) {
		t.Fatalf("expected %s. got %s", exp, nw)
	}
}

func createInterface(t *testing.T, name string, nws ...string) {
	// Add interface
	link := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: "test",
		},
	}
	bips := []*net.IPNet{}
	for _, nw := range nws {
		bip, err := types.ParseCIDR(nw)
		if err != nil {
			t.Fatal(err)
		}
		bips = append(bips, bip)
	}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatalf("Failed to create interface via netlink: %v", err)
	}
	for _, bip := range bips {
		if err := netlink.AddrAdd(link, &netlink.Addr{IPNet: bip}); err != nil {
			t.Fatal(err)
		}
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
}
