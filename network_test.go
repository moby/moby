package docker

import (
	"net"
	"os"
	"testing"
)

func TestIptables(t *testing.T) {
	if err := iptables("-L"); err != nil {
		t.Fatal(err)
	}
	path := os.Getenv("PATH")
	os.Setenv("PATH", "")
	defer os.Setenv("PATH", path)
	if err := iptables("-L"); err == nil {
		t.Fatal("Not finding iptables in the PATH should cause an error")
	}
}

func TestNetworkRange(t *testing.T) {
	// Simple class C test
	_, network, _ := net.ParseCIDR("192.168.0.1/24")
	first, last := networkRange(network)
	if !first.Equal(net.ParseIP("192.168.0.0")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("192.168.0.255")) {
		t.Error(last.String())
	}
	if size := networkSize(network.Mask); size != 256 {
		t.Error(size)
	}

	// Class A test
	_, network, _ = net.ParseCIDR("10.0.0.1/8")
	first, last = networkRange(network)
	if !first.Equal(net.ParseIP("10.0.0.0")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("10.255.255.255")) {
		t.Error(last.String())
	}
	if size := networkSize(network.Mask); size != 16777216 {
		t.Error(size)
	}

	// Class A, random IP address
	_, network, _ = net.ParseCIDR("10.1.2.3/8")
	first, last = networkRange(network)
	if !first.Equal(net.ParseIP("10.0.0.0")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("10.255.255.255")) {
		t.Error(last.String())
	}

	// 32bit mask
	_, network, _ = net.ParseCIDR("10.1.2.3/32")
	first, last = networkRange(network)
	if !first.Equal(net.ParseIP("10.1.2.3")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("10.1.2.3")) {
		t.Error(last.String())
	}
	if size := networkSize(network.Mask); size != 1 {
		t.Error(size)
	}

	// 31bit mask
	_, network, _ = net.ParseCIDR("10.1.2.3/31")
	first, last = networkRange(network)
	if !first.Equal(net.ParseIP("10.1.2.2")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("10.1.2.3")) {
		t.Error(last.String())
	}
	if size := networkSize(network.Mask); size != 2 {
		t.Error(size)
	}

	// 26bit mask
	_, network, _ = net.ParseCIDR("10.1.2.3/26")
	first, last = networkRange(network)
	if !first.Equal(net.ParseIP("10.1.2.0")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("10.1.2.63")) {
		t.Error(last.String())
	}
	if size := networkSize(network.Mask); size != 64 {
		t.Error(size)
	}
}

func TestConversion(t *testing.T) {
	ip := net.ParseIP("127.0.0.1")
	i := ipToInt(ip)
	if i == 0 {
		t.Fatal("converted to zero")
	}
	conv := intToIp(i)
	if !ip.Equal(conv) {
		t.Error(conv.String())
	}
}

func TestIPAllocator(t *testing.T) {
	expectedIPs := []net.IP{
		0: net.IPv4(127, 0, 0, 2),
		1: net.IPv4(127, 0, 0, 3),
		2: net.IPv4(127, 0, 0, 4),
		3: net.IPv4(127, 0, 0, 5),
		4: net.IPv4(127, 0, 0, 6),
	}

	gwIP, n, _ := net.ParseCIDR("127.0.0.1/29")
	alloc := newIPAllocator(&net.IPNet{IP: gwIP, Mask: n.Mask})
	// Pool after initialisation (f = free, u = used)
	// 2(f) - 3(f) - 4(f) - 5(f) - 6(f)
	//  ↑

	// Check that we get 5 IPs, from 127.0.0.2–127.0.0.6, in that
	// order.
	for i := 0; i < 5; i++ {
		ip, err := alloc.Acquire()
		if err != nil {
			t.Fatal(err)
		}

		assertIPEquals(t, expectedIPs[i], ip)
	}
	// Before loop begin
	// 2(f) - 3(f) - 4(f) - 5(f) - 6(f)
	//  ↑

	// After i = 0
	// 2(u) - 3(f) - 4(f) - 5(f) - 6(f)
	//         ↑

	// After i = 1
	// 2(u) - 3(u) - 4(f) - 5(f) - 6(f)
	//                ↑

	// After i = 2
	// 2(u) - 3(u) - 4(u) - 5(f) - 6(f)
	//                       ↑

	// After i = 3
	// 2(u) - 3(u) - 4(u) - 5(u) - 6(f)
	//                              ↑

	// After i = 4
	// 2(u) - 3(u) - 4(u) - 5(u) - 6(u)
	//  ↑

	// Check that there are no more IPs
	_, err := alloc.Acquire()
	if err == nil {
		t.Fatal("There shouldn't be any IP addresses at this point")
	}

	// Release some IPs in non-sequential order
	alloc.Release(expectedIPs[3])
	// 2(u) - 3(u) - 4(u) - 5(f) - 6(u)
	//                       ↑

	alloc.Release(expectedIPs[2])
	// 2(u) - 3(u) - 4(f) - 5(f) - 6(u)
	//                       ↑

	alloc.Release(expectedIPs[4])
	// 2(u) - 3(u) - 4(f) - 5(f) - 6(f)
	//                       ↑

	// Make sure that IPs are reused in sequential order, starting
	// with the first released IP
	newIPs := make([]net.IP, 3)
	for i := 0; i < 3; i++ {
		ip, err := alloc.Acquire()
		if err != nil {
			t.Fatal(err)
		}

		newIPs[i] = ip
	}
	// Before loop begin
	// 2(u) - 3(u) - 4(f) - 5(f) - 6(f)
	//                       ↑

	// After i = 0
	// 2(u) - 3(u) - 4(f) - 5(u) - 6(f)
	//                              ↑

	// After i = 1
	// 2(u) - 3(u) - 4(f) - 5(u) - 6(u)
	//                ↑

	// After i = 2
	// 2(u) - 3(u) - 4(u) - 5(u) - 6(u)
	//                       ↑

	assertIPEquals(t, expectedIPs[3], newIPs[0])
	assertIPEquals(t, expectedIPs[4], newIPs[1])
	assertIPEquals(t, expectedIPs[2], newIPs[2])

	_, err = alloc.Acquire()
	if err == nil {
		t.Fatal("There shouldn't be any IP addresses at this point")
	}
}

func assertIPEquals(t *testing.T, ip1, ip2 net.IP) {
	if !ip1.Equal(ip2) {
		t.Fatalf("Expected IP %s, got %s", ip1, ip2)
	}
}
