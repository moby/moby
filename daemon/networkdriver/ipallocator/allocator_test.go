package ipallocator

import (
	"fmt"
	"net"
	"testing"
)

func reset() {
	allocatedIPs = networkSet{}
	availableIPs = networkPool{}
}

func TestRequestNewIps(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{192, 168, 0, 1},
		Mask: []byte{255, 255, 255, 0},
	}

	for i := 2; i < 10; i++ {
		ip, err := RequestIP(network, nil)
		if err != nil {
			t.Fatal(err)
		}

		if expected := fmt.Sprintf("192.168.0.%d", i); ip.String() != expected {
			t.Fatalf("Expected ip %s got %s", expected, ip.String())
		}
	}
}

func TestReleaseIp(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{192, 168, 0, 1},
		Mask: []byte{255, 255, 255, 0},
	}

	ip, err := RequestIP(network, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := ReleaseIP(network, ip); err != nil {
		t.Fatal(err)
	}
}

func TestGetReleasedIp(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{192, 168, 0, 1},
		Mask: []byte{255, 255, 255, 0},
	}

	ip, err := RequestIP(network, nil)
	if err != nil {
		t.Fatal(err)
	}

	value := intToIP(ipToInt(ip) + 1).String()

	if err := ReleaseIP(network, ip); err != nil {
		t.Fatal(err)
	}

	ip, err = RequestIP(network, nil)
	if err != nil {
		t.Fatal(err)
	}

	if ip.String() != value {
		t.Fatalf("Expected to receive the next ip %s got %s", value, ip.String())
	}
}

func TestGetReleasedIps(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{192, 168, 0, 1},
		Mask: []byte{255, 255, 255, 0},
	}

	ip, err := RequestIP(network, nil)
	if err != nil {
		t.Fatal(err)
	}

	value := ip.String()
	if err := ReleaseIP(network, ip); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 252; i++ {
		_, err = RequestIP(network, nil)
		if err != nil {
			t.Fatal(err)
		}

		err = ReleaseIP(network, ip)
		if err != nil {
			t.Fatal(err)
		}
	}

	ip, err = RequestIP(network, nil)
	if err != nil {
		t.Fatal(err)
	}

	if ip.String() != value {
		t.Fatalf("Expected to receive same ip %s got %s", value, ip.String())
	}
}

func TestRequesetSpecificIp(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{192, 168, 0, 1},
		Mask: []byte{255, 255, 255, 0},
	}

	ip := net.ParseIP("192.168.1.5")

	if _, err := RequestIP(network, &ip); err != nil {
		t.Fatal(err)
	}
}

func TestConversion(t *testing.T) {
	ip := net.ParseIP("127.0.0.1")
	i := ipToInt(&ip)
	if i == 0 {
		t.Fatal("converted to zero")
	}
	conv := intToIP(i)
	if !ip.Equal(*conv) {
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
	network := &net.IPNet{IP: gwIP, Mask: n.Mask}
	// Pool after initialisation (f = free, u = used)
	// 2(f) - 3(f) - 4(f) - 5(f) - 6(f)
	//  ↑

	// Check that we get 5 IPs, from 127.0.0.2–127.0.0.6, in that
	// order.
	for i := 0; i < 5; i++ {
		ip, err := RequestIP(network, nil)
		if err != nil {
			t.Fatal(err)
		}

		assertIPEquals(t, &expectedIPs[i], ip)
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
	ip, err := RequestIP(network, nil)
	if err == nil {
		t.Fatalf("There shouldn't be any IP addresses at this point, got %s\n", ip)
	}

	// Release some IPs in non-sequential order
	if err := ReleaseIP(network, &expectedIPs[3]); err != nil {
		t.Fatal(err)
	}
	// 2(u) - 3(u) - 4(u) - 5(f) - 6(u)
	//                       ↑

	if err := ReleaseIP(network, &expectedIPs[2]); err != nil {
		t.Fatal(err)
	}
	// 2(u) - 3(u) - 4(f) - 5(f) - 6(u)
	//                       ↑

	if err := ReleaseIP(network, &expectedIPs[4]); err != nil {
		t.Fatal(err)
	}
	// 2(u) - 3(u) - 4(f) - 5(f) - 6(f)
	//                       ↑

	// Make sure that IPs are reused in the order they were released
	newIPs := make([]*net.IP, 3)
	for i := 0; i < 3; i++ {
		ip, err := RequestIP(network, nil)
		if err != nil {
			t.Fatal(err)
		}

		newIPs[i] = ip
	}

	// the assertions are in the same order as calls to ReleaseIP
	assertIPEquals(t, &expectedIPs[3], newIPs[0])
	assertIPEquals(t, &expectedIPs[2], newIPs[1])
	assertIPEquals(t, &expectedIPs[4], newIPs[2])

	_, err = RequestIP(network, nil)
	if err == nil {
		t.Fatal("There shouldn't be any IP addresses at this point")
	}
}

func TestAllocateFirstIP(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{192, 168, 0, 0},
		Mask: []byte{255, 255, 255, 0},
	}

	firstIP := network.IP.To4().Mask(network.Mask)
	first := ipToInt(&firstIP) + 1

	ip, err := RequestIP(network, nil)
	if err != nil {
		t.Fatal(err)
	}
	allocated := ipToInt(ip)

	if allocated == first {
		t.Fatalf("allocated ip should not equal first ip: %d == %d", first, allocated)
	}
}

func assertIPEquals(t *testing.T, ip1, ip2 *net.IP) {
	if !ip1.Equal(*ip2) {
		t.Fatalf("Expected IP %s, got %s", ip1, ip2)
	}
}
