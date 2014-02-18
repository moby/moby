package networkdriver

import (
	"github.com/dotcloud/docker/pkg/netlink"
	"net"
	"testing"
)

func TestNonOverlapingNameservers(t *testing.T) {
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

func TestOverlapingNameservers(t *testing.T) {
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
	orig := networkGetRoutesFct
	defer func() {
		networkGetRoutesFct = orig
	}()
	networkGetRoutesFct = func() ([]netlink.Route, error) {
		routesData := []string{"10.0.2.0/32", "10.0.3.0/24", "10.0.42.0/24", "172.16.42.0/24", "192.168.142.0/24"}

		routes := []netlink.Route{}
		for _, addr := range routesData {
			_, netX, _ := net.ParseCIDR(addr)
			routes = append(routes, netlink.Route{IPNet: netX})
		}
		return routes, nil
	}

	_, netX, _ := net.ParseCIDR("172.16.0.1/24")
	if err := CheckRouteOverlaps(netX); err != nil {
		t.Fatal(err)
	}

	_, netX, _ = net.ParseCIDR("10.0.2.0/24")
	if err := CheckRouteOverlaps(netX); err == nil {
		t.Fatalf("10.0.2.0/24 and 10.0.2.0 should overlap but it doesn't")
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
	//netY starts at same IP and ends within netX
	AssertOverlap("172.16.0.1/24", "172.16.0.1/25", t)
	AssertOverlap("2001:db8::1/64", "2001:db8::1/65", t)
	//netY starts within netX and ends at same IP
	AssertOverlap("172.16.0.1/24", "172.16.0.128/25", t)
	AssertOverlap("2001:db8::1/64", "2001:db8::1/65", t)
	//netY starts and ends within netX
	AssertOverlap("172.16.0.1/24", "172.16.0.64/25", t)
	AssertOverlap("2001:db8::1/64", "2001:db8::64/65", t)
	//netY starts at same IP and ends outside of netX
	AssertOverlap("172.16.0.1/24", "172.16.0.1/23", t)
	AssertOverlap("2001:db8::1/64", "2001:db8::1/48", t)
	//netY starts before and ends at same IP of netX
	AssertOverlap("172.16.1.1/24", "172.16.0.1/23", t)
	AssertOverlap("2001:db8:0:1::1/64", "2001:db8::1/63", t)
	//netY starts before and ends outside of netX
	AssertOverlap("172.16.1.1/24", "172.16.0.1/22", t)
	AssertOverlap("2001:db8::1/64", "2001:db8::1/48", t)
	//netY starts and ends before netX
	AssertNoOverlap("172.16.1.1/25", "172.16.0.1/24", t)
	AssertNoOverlap("2001:db8:2::1/64", "2001:db8::1/64", t)
	//netX starts and ends before netY
	AssertNoOverlap("172.16.1.1/25", "172.16.2.1/24", t)
	AssertNoOverlap("2001:db8::1/64", "2001:db8:2::1/64", t)
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
	if size := NetworkSize(network.Mask); size != 256 {
		t.Error(size)
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
	if size := NetworkSize(network.Mask); size != 16777216 {
		t.Error(size)
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
	if size := NetworkSize(network.Mask); size != 1 {
		t.Error(size)
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
	if size := NetworkSize(network.Mask); size != 2 {
		t.Error(size)
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
	if size := NetworkSize(network.Mask); size != 64 {
		t.Error(size)
	}


	// IPv6
	// 48bit mask
	_, network, _ = net.ParseCIDR("2001:db8::1/48")
	first, last = NetworkRange(network)
	if !first.Equal(net.ParseIP("2001:db8::")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("2001:db8::ffff:ffff:ffff:ffff:ffff")) {
		t.Error(last.String())
	}
	if size, size2 := NetworkSize6(network.Mask); size != uint64(65535) || size2 != uint64(18446744073709551615) {
		t.Error(size)
	}

	// 64bit mask
	_, network, _ = net.ParseCIDR("2001:db8::1/64")
	first, last = NetworkRange(network)
	if !first.Equal(net.ParseIP("2001:db8::")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("2001:db8::ffff:ffff:ffff:ffff")) {
		t.Error(last.String())
	}
	if size, size2 := NetworkSize6(network.Mask); size != 0 || size2 != uint64(18446744073709551615) {
		t.Error(size)
	}

	// 127bit mask
	_, network, _ = net.ParseCIDR("2001:db8::1/127")
	first, last = NetworkRange(network)
	if !first.Equal(net.ParseIP("2001:db8::")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("2001:db8::1")) {
		t.Error(last.String())
	}
	if size, size2 := NetworkSize6(network.Mask); size != 0 || size2 != uint64(1) {
		t.Error(size)
	}

	// 128bit mask
	_, network, _ = net.ParseCIDR("2001:db8::1/128")
	first, last = NetworkRange(network)
	if !first.Equal(net.ParseIP("2001:db8::1")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("2001:db8::1")) {
		t.Error(last.String())
	}
	if size, size2 := NetworkSize6(network.Mask); size != 0 || size2 != 0 {
		t.Error(size)
	}
}
