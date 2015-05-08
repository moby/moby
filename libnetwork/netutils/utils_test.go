package netutils

import (
	"bytes"
	"net"
	"testing"

	"github.com/vishvananda/netlink"
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
	networkGetRoutesFct = func(netlink.Link, int) ([]netlink.Route, error) {
		routesData := []string{"10.0.2.0/32", "10.0.3.0/24", "10.0.42.0/24", "172.16.42.0/24", "192.168.142.0/24"}

		routes := []netlink.Route{}
		for _, addr := range routesData {
			_, netX, _ := net.ParseCIDR(addr)
			routes = append(routes, netlink.Route{Dst: netX})
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
	//netY starts within netX and ends at same IP
	AssertOverlap("172.16.0.1/24", "172.16.0.128/25", t)
	//netY starts and ends within netX
	AssertOverlap("172.16.0.1/24", "172.16.0.64/25", t)
	//netY starts at same IP and ends outside of netX
	AssertOverlap("172.16.0.1/24", "172.16.0.1/23", t)
	//netY starts before and ends at same IP of netX
	AssertOverlap("172.16.1.1/24", "172.16.0.1/23", t)
	//netY starts before and ends outside of netX
	AssertOverlap("172.16.1.1/24", "172.16.0.1/22", t)
	//netY starts and ends before netX
	AssertNoOverlap("172.16.1.1/25", "172.16.0.1/24", t)
	//netX starts and ends before netY
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
	name1, err := GenerateRandomName("veth", 7)
	if err != nil {
		t.Fatal(err)
	}
	// veth plus generated append equals a len of 11
	if len(name1) != 11 {
		t.Fatalf("Expected 11 characters, instead received %d characters", len(name1))
	}
	name2, err := GenerateRandomName("veth", 7)
	if err != nil {
		t.Fatal(err)
	}
	// Fail if the random generated names equal one another
	if name1 == name2 {
		t.Fatalf("Expected differing values but received %s and %s", name1, name2)
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

func TestCompareIPNet(t *testing.T) {
	if CompareIPNet(nil, nil) == false {
		t.Fatalf("Failed to detect two nil net.IPNets are equal")
	}

	_, net1, _ := net.ParseCIDR("192.168.30.22/24")
	if CompareIPNet(net1, net1) == false {
		t.Fatalf("Failed to detect same net.IPNet pointers equality")
	}

	_, net2, _ := net.ParseCIDR("192.168.30.22/24")
	if CompareIPNet(net1, net2) == false {
		t.Fatalf("Failed to detect same net.IPNet object equality")
	}

	_, net3, _ := net.ParseCIDR("192.168.30.33/24")
	if CompareIPNet(net1, net3) == false {
		t.Fatalf("Failed to detect semantically equivalent net.IPNets")
	}

	_, net3, _ = net.ParseCIDR("192.168.31.33/24")
	if CompareIPNet(net2, net3) == true {
		t.Fatalf("Failed to detect different net.IPNets")
	}
}

func TestIPCopyFunctions(t *testing.T) {
	ip := net.ParseIP("172.28.30.134")
	cp := GetIPCopy(ip)

	if !ip.Equal(cp) {
		t.Fatalf("Failed to return a copy of net.IP")
	}

	if &ip == &cp {
		t.Fatalf("Failed to return a true copy of net.IP")
	}
}

func TestNetIPCopyFunctions(t *testing.T) {
	_, net, _ := net.ParseCIDR("192.168.30.23/24")
	cp := GetIPNetCopy(net)

	if CompareIPNet(net, cp) == false {
		t.Fatalf("Failed to return a copy of net.IPNet")
	}

	if net == cp {
		t.Fatalf("Failed to return a true copy of net.IPNet")
	}
}

func TestPortBindingEqual(t *testing.T) {
	pb1 := &PortBinding{
		Proto:    TCP,
		IP:       net.ParseIP("172.17.0.1"),
		Port:     80,
		HostIP:   net.ParseIP("192.168.100.1"),
		HostPort: 8080,
	}

	pb2 := &PortBinding{
		Proto:    UDP,
		IP:       net.ParseIP("172.17.0.1"),
		Port:     22,
		HostIP:   net.ParseIP("192.168.100.1"),
		HostPort: 2222,
	}
	if !pb1.Equal(pb1) {
		t.Fatalf("PortBinding.Equal() returned false negative")
	}

	if pb1.Equal(nil) {
		t.Fatalf("PortBinding.Equal() returned false negative")
	}

	if pb1.Equal(pb2) {
		t.Fatalf("PortBinding.Equal() returned false positive")
	}

	if pb1.Equal(pb2) != pb2.Equal(pb1) {
		t.Fatalf("PortBinding.Equal() failed commutative check")
	}
}

func TestPortBindingGetCopy(t *testing.T) {
	pb := &PortBinding{
		Proto:    TCP,
		IP:       net.ParseIP("172.17.0.1"),
		Port:     80,
		HostIP:   net.ParseIP("192.168.100.1"),
		HostPort: 8080,
	}
	cp := pb.GetCopy()

	if !pb.Equal(&cp) {
		t.Fatalf("Failed to return a copy of PortBinding")
	}

	if pb == &cp {
		t.Fatalf("Failed to return a true copy of PortBinding")
	}
}

func TestPortBindingContainerAddr(t *testing.T) {
	pb := PortBinding{
		Proto:    TCP,
		IP:       net.ParseIP("172.17.0.1"),
		Port:     80,
		HostIP:   net.ParseIP("192.168.100.1"),
		HostPort: 8080,
	}

	container, err := pb.ContainerAddr()

	if err != nil {
		t.Fatal(err)
	}

	switch netAddr := container.(type) {
	case *net.TCPAddr:
		if !pb.IP.Equal(netAddr.IP) {
			t.Fatalf("PortBinding.ContainerAddr() Failed to return a ContainerAddr")
		}
		if int(pb.Port) != netAddr.Port {
			t.Fatalf("PortBinding.ContainerAddr() Failed to return a ContainerAddr")
		}
	case *net.UDPAddr:
		t.Fatalf("PortBinding.ContainerAddr() Failed to check correct proto")
	}
}
