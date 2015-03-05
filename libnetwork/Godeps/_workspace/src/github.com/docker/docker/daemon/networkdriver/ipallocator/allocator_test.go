package ipallocator

import (
	"fmt"
	"math/big"
	"net"
	"testing"
)

func reset() {
	allocatedIPs = networkSet{}
}

func TestConversion(t *testing.T) {
	ip := net.ParseIP("127.0.0.1")
	i := ipToBigInt(ip)
	if i.Cmp(big.NewInt(0x7f000001)) != 0 {
		t.Fatal("incorrect conversion")
	}
	conv := bigIntToIP(i)
	if !ip.Equal(conv) {
		t.Error(conv.String())
	}
}

func TestConversionIPv6(t *testing.T) {
	ip := net.ParseIP("2a00:1450::1")
	ip2 := net.ParseIP("2a00:1450::2")
	ip3 := net.ParseIP("2a00:1450::1:1")
	i := ipToBigInt(ip)
	val, success := big.NewInt(0).SetString("2a001450000000000000000000000001", 16)
	if !success {
		t.Fatal("Hex-String to BigInt conversion failed.")
	}
	if i.Cmp(val) != 0 {
		t.Fatal("incorrent conversion")
	}

	conv := bigIntToIP(i)
	conv2 := bigIntToIP(big.NewInt(0).Add(i, big.NewInt(1)))
	conv3 := bigIntToIP(big.NewInt(0).Add(i, big.NewInt(0x10000)))

	if !ip.Equal(conv) {
		t.Error("2a00:1450::1 should be equal to " + conv.String())
	}
	if !ip2.Equal(conv2) {
		t.Error("2a00:1450::2 should be equal to " + conv2.String())
	}
	if !ip3.Equal(conv3) {
		t.Error("2a00:1450::1:1 should be equal to " + conv3.String())
	}
}

func TestRequestNewIps(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{192, 168, 0, 1},
		Mask: []byte{255, 255, 255, 0},
	}

	var ip net.IP
	var err error

	for i := 1; i < 10; i++ {
		ip, err = RequestIP(network, nil)
		if err != nil {
			t.Fatal(err)
		}

		if expected := fmt.Sprintf("192.168.0.%d", i); ip.String() != expected {
			t.Fatalf("Expected ip %s got %s", expected, ip.String())
		}
	}
	value := bigIntToIP(big.NewInt(0).Add(ipToBigInt(ip), big.NewInt(1))).String()
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

func TestRequestNewIpV6(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{0x2a, 0x00, 0x14, 0x50, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		Mask: []byte{255, 255, 255, 255, 255, 255, 255, 255, 0, 0, 0, 0, 0, 0, 0, 0}, // /64 netmask
	}

	var ip net.IP
	var err error
	for i := 1; i < 10; i++ {
		ip, err = RequestIP(network, nil)
		if err != nil {
			t.Fatal(err)
		}

		if expected := fmt.Sprintf("2a00:1450::%d", i); ip.String() != expected {
			t.Fatalf("Expected ip %s got %s", expected, ip.String())
		}
	}
	value := bigIntToIP(big.NewInt(0).Add(ipToBigInt(ip), big.NewInt(1))).String()
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

func TestReleaseIpV6(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{0x2a, 0x00, 0x14, 0x50, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		Mask: []byte{255, 255, 255, 255, 255, 255, 255, 255, 0, 0, 0, 0, 0, 0, 0, 0}, // /64 netmask
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

	value := ip.String()
	if err := ReleaseIP(network, ip); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 253; i++ {
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

func TestGetReleasedIpV6(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{0x2a, 0x00, 0x14, 0x50, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		Mask: []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 0},
	}

	ip, err := RequestIP(network, nil)
	if err != nil {
		t.Fatal(err)
	}

	value := ip.String()
	if err := ReleaseIP(network, ip); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 253; i++ {
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

func TestRequestSpecificIp(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{192, 168, 0, 1},
		Mask: []byte{255, 255, 255, 224},
	}

	ip := net.ParseIP("192.168.0.5")

	// Request a "good" IP.
	if _, err := RequestIP(network, ip); err != nil {
		t.Fatal(err)
	}

	// Request the same IP again.
	if _, err := RequestIP(network, ip); err != ErrIPAlreadyAllocated {
		t.Fatalf("Got the same IP twice: %#v", err)
	}

	// Request an out of range IP.
	if _, err := RequestIP(network, net.ParseIP("192.168.0.42")); err != ErrIPOutOfRange {
		t.Fatalf("Got an out of range IP: %#v", err)
	}
}

func TestRequestSpecificIpV6(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{0x2a, 0x00, 0x14, 0x50, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		Mask: []byte{255, 255, 255, 255, 255, 255, 255, 255, 0, 0, 0, 0, 0, 0, 0, 0}, // /64 netmask
	}

	ip := net.ParseIP("2a00:1450::5")

	// Request a "good" IP.
	if _, err := RequestIP(network, ip); err != nil {
		t.Fatal(err)
	}

	// Request the same IP again.
	if _, err := RequestIP(network, ip); err != ErrIPAlreadyAllocated {
		t.Fatalf("Got the same IP twice: %#v", err)
	}

	// Request an out of range IP.
	if _, err := RequestIP(network, net.ParseIP("2a00:1500::1")); err != ErrIPOutOfRange {
		t.Fatalf("Got an out of range IP: %#v", err)
	}
}

func TestIPAllocator(t *testing.T) {
	expectedIPs := []net.IP{
		0: net.IPv4(127, 0, 0, 1),
		1: net.IPv4(127, 0, 0, 2),
		2: net.IPv4(127, 0, 0, 3),
		3: net.IPv4(127, 0, 0, 4),
		4: net.IPv4(127, 0, 0, 5),
		5: net.IPv4(127, 0, 0, 6),
	}

	gwIP, n, _ := net.ParseCIDR("127.0.0.1/29")

	network := &net.IPNet{IP: gwIP, Mask: n.Mask}
	// Pool after initialisation (f = free, u = used)
	// 1(f) - 2(f) - 3(f) - 4(f) - 5(f) - 6(f)
	//  ↑

	// Check that we get 6 IPs, from 127.0.0.1–127.0.0.6, in that
	// order.
	for i := 0; i < 6; i++ {
		ip, err := RequestIP(network, nil)
		if err != nil {
			t.Fatal(err)
		}

		assertIPEquals(t, expectedIPs[i], ip)
	}
	// Before loop begin
	// 1(f) - 2(f) - 3(f) - 4(f) - 5(f) - 6(f)
	//  ↑

	// After i = 0
	// 1(u) - 2(f) - 3(f) - 4(f) - 5(f) - 6(f)
	//         ↑

	// After i = 1
	// 1(u) - 2(u) - 3(f) - 4(f) - 5(f) - 6(f)
	//                ↑

	// After i = 2
	// 1(u) - 2(u) - 3(u) - 4(f) - 5(f) - 6(f)
	//                       ↑

	// After i = 3
	// 1(u) - 2(u) - 3(u) - 4(u) - 5(f) - 6(f)
	//                              ↑

	// After i = 4
	// 1(u) - 2(u) - 3(u) - 4(u) - 5(u) - 6(f)
	//                                     ↑

	// After i = 5
	// 1(u) - 2(u) - 3(u) - 4(u) - 5(u) - 6(u)
	//  ↑

	// Check that there are no more IPs
	ip, err := RequestIP(network, nil)
	if err == nil {
		t.Fatalf("There shouldn't be any IP addresses at this point, got %s\n", ip)
	}

	// Release some IPs in non-sequential order
	if err := ReleaseIP(network, expectedIPs[3]); err != nil {
		t.Fatal(err)
	}
	// 1(u) - 2(u) - 3(u) - 4(f) - 5(u) - 6(u)
	//                       ↑

	if err := ReleaseIP(network, expectedIPs[2]); err != nil {
		t.Fatal(err)
	}
	// 1(u) - 2(u) - 3(f) - 4(f) - 5(u) - 6(u)
	//                ↑

	if err := ReleaseIP(network, expectedIPs[4]); err != nil {
		t.Fatal(err)
	}
	// 1(u) - 2(u) - 3(f) - 4(f) - 5(f) - 6(u)
	//                              ↑

	// Make sure that IPs are reused in sequential order, starting
	// with the first released IP
	newIPs := make([]net.IP, 3)
	for i := 0; i < 3; i++ {
		ip, err := RequestIP(network, nil)
		if err != nil {
			t.Fatal(err)
		}

		newIPs[i] = ip
	}
	assertIPEquals(t, expectedIPs[2], newIPs[0])
	assertIPEquals(t, expectedIPs[3], newIPs[1])
	assertIPEquals(t, expectedIPs[4], newIPs[2])

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
	first := big.NewInt(0).Add(ipToBigInt(firstIP), big.NewInt(1))

	ip, err := RequestIP(network, nil)
	if err != nil {
		t.Fatal(err)
	}
	allocated := ipToBigInt(ip)

	if allocated == first {
		t.Fatalf("allocated ip should not equal first ip: %d == %d", first, allocated)
	}
}

func TestAllocateAllIps(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{192, 168, 0, 1},
		Mask: []byte{255, 255, 255, 0},
	}

	var (
		current, first net.IP
		err            error
		isFirst        = true
	)

	for err == nil {
		current, err = RequestIP(network, nil)
		if isFirst {
			first = current
			isFirst = false
		}
	}

	if err != ErrNoAvailableIPs {
		t.Fatal(err)
	}

	if _, err := RequestIP(network, nil); err != ErrNoAvailableIPs {
		t.Fatal(err)
	}

	if err := ReleaseIP(network, first); err != nil {
		t.Fatal(err)
	}

	again, err := RequestIP(network, nil)
	if err != nil {
		t.Fatal(err)
	}

	assertIPEquals(t, first, again)

	// ensure that alloc.last == alloc.begin won't result in dead loop
	if _, err := RequestIP(network, nil); err != ErrNoAvailableIPs {
		t.Fatal(err)
	}

	// Test by making alloc.last the only free ip and ensure we get it back
	// #1. first of the range, (alloc.last == ipToInt(first) already)
	if err := ReleaseIP(network, first); err != nil {
		t.Fatal(err)
	}

	ret, err := RequestIP(network, nil)
	if err != nil {
		t.Fatal(err)
	}

	assertIPEquals(t, first, ret)

	// #2. last of the range, note that current is the last one
	last := net.IPv4(192, 168, 0, 254)
	setLastTo(t, network, last)

	ret, err = RequestIP(network, nil)
	if err != nil {
		t.Fatal(err)
	}

	assertIPEquals(t, last, ret)

	// #3. middle of the range
	mid := net.IPv4(192, 168, 0, 7)
	setLastTo(t, network, mid)

	ret, err = RequestIP(network, nil)
	if err != nil {
		t.Fatal(err)
	}

	assertIPEquals(t, mid, ret)
}

// make sure the pool is full when calling setLastTo.
// we don't cheat here
func setLastTo(t *testing.T, network *net.IPNet, ip net.IP) {
	if err := ReleaseIP(network, ip); err != nil {
		t.Fatal(err)
	}

	ret, err := RequestIP(network, nil)
	if err != nil {
		t.Fatal(err)
	}

	assertIPEquals(t, ip, ret)

	if err := ReleaseIP(network, ip); err != nil {
		t.Fatal(err)
	}
}

func TestAllocateDifferentSubnets(t *testing.T) {
	defer reset()
	network1 := &net.IPNet{
		IP:   []byte{192, 168, 0, 1},
		Mask: []byte{255, 255, 255, 0},
	}
	network2 := &net.IPNet{
		IP:   []byte{127, 0, 0, 1},
		Mask: []byte{255, 255, 255, 0},
	}
	network3 := &net.IPNet{
		IP:   []byte{0x2a, 0x00, 0x14, 0x50, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		Mask: []byte{255, 255, 255, 255, 255, 255, 255, 255, 0, 0, 0, 0, 0, 0, 0, 0}, // /64 netmask
	}
	network4 := &net.IPNet{
		IP:   []byte{0x2a, 0x00, 0x16, 0x32, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		Mask: []byte{255, 255, 255, 255, 255, 255, 255, 255, 0, 0, 0, 0, 0, 0, 0, 0}, // /64 netmask
	}
	expectedIPs := []net.IP{
		0: net.IPv4(192, 168, 0, 1),
		1: net.IPv4(192, 168, 0, 2),
		2: net.IPv4(127, 0, 0, 1),
		3: net.IPv4(127, 0, 0, 2),
		4: net.ParseIP("2a00:1450::1"),
		5: net.ParseIP("2a00:1450::2"),
		6: net.ParseIP("2a00:1450::3"),
		7: net.ParseIP("2a00:1632::1"),
		8: net.ParseIP("2a00:1632::2"),
	}

	ip11, err := RequestIP(network1, nil)
	if err != nil {
		t.Fatal(err)
	}
	ip12, err := RequestIP(network1, nil)
	if err != nil {
		t.Fatal(err)
	}
	ip21, err := RequestIP(network2, nil)
	if err != nil {
		t.Fatal(err)
	}
	ip22, err := RequestIP(network2, nil)
	if err != nil {
		t.Fatal(err)
	}
	ip31, err := RequestIP(network3, nil)
	if err != nil {
		t.Fatal(err)
	}
	ip32, err := RequestIP(network3, nil)
	if err != nil {
		t.Fatal(err)
	}
	ip33, err := RequestIP(network3, nil)
	if err != nil {
		t.Fatal(err)
	}
	ip41, err := RequestIP(network4, nil)
	if err != nil {
		t.Fatal(err)
	}
	ip42, err := RequestIP(network4, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertIPEquals(t, expectedIPs[0], ip11)
	assertIPEquals(t, expectedIPs[1], ip12)
	assertIPEquals(t, expectedIPs[2], ip21)
	assertIPEquals(t, expectedIPs[3], ip22)
	assertIPEquals(t, expectedIPs[4], ip31)
	assertIPEquals(t, expectedIPs[5], ip32)
	assertIPEquals(t, expectedIPs[6], ip33)
	assertIPEquals(t, expectedIPs[7], ip41)
	assertIPEquals(t, expectedIPs[8], ip42)
}

func TestRegisterBadTwice(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{192, 168, 1, 1},
		Mask: []byte{255, 255, 255, 0},
	}
	subnet := &net.IPNet{
		IP:   []byte{192, 168, 1, 8},
		Mask: []byte{255, 255, 255, 248},
	}

	if err := RegisterSubnet(network, subnet); err != nil {
		t.Fatal(err)
	}
	subnet = &net.IPNet{
		IP:   []byte{192, 168, 1, 16},
		Mask: []byte{255, 255, 255, 248},
	}
	if err := RegisterSubnet(network, subnet); err != ErrNetworkAlreadyRegistered {
		t.Fatalf("Expecteded ErrNetworkAlreadyRegistered error, got %v", err)
	}
}

func TestRegisterBadRange(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{192, 168, 1, 1},
		Mask: []byte{255, 255, 255, 0},
	}
	subnet := &net.IPNet{
		IP:   []byte{192, 168, 1, 1},
		Mask: []byte{255, 255, 0, 0},
	}
	if err := RegisterSubnet(network, subnet); err != ErrBadSubnet {
		t.Fatalf("Expected ErrBadSubnet error, got %v", err)
	}
}

func TestAllocateFromRange(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{192, 168, 0, 1},
		Mask: []byte{255, 255, 255, 0},
	}
	// 192.168.1.9 - 192.168.1.14
	subnet := &net.IPNet{
		IP:   []byte{192, 168, 0, 8},
		Mask: []byte{255, 255, 255, 248},
	}

	if err := RegisterSubnet(network, subnet); err != nil {
		t.Fatal(err)
	}
	expectedIPs := []net.IP{
		0: net.IPv4(192, 168, 0, 9),
		1: net.IPv4(192, 168, 0, 10),
		2: net.IPv4(192, 168, 0, 11),
		3: net.IPv4(192, 168, 0, 12),
		4: net.IPv4(192, 168, 0, 13),
		5: net.IPv4(192, 168, 0, 14),
	}
	for _, ip := range expectedIPs {
		rip, err := RequestIP(network, nil)
		if err != nil {
			t.Fatal(err)
		}
		assertIPEquals(t, ip, rip)
	}

	if _, err := RequestIP(network, nil); err != ErrNoAvailableIPs {
		t.Fatalf("Expected ErrNoAvailableIPs error, got %v", err)
	}
	for _, ip := range expectedIPs {
		ReleaseIP(network, ip)
		rip, err := RequestIP(network, nil)
		if err != nil {
			t.Fatal(err)
		}
		assertIPEquals(t, ip, rip)
	}
}

func assertIPEquals(t *testing.T, ip1, ip2 net.IP) {
	if !ip1.Equal(ip2) {
		t.Fatalf("Expected IP %s, got %s", ip1, ip2)
	}
}

func BenchmarkRequestIP(b *testing.B) {
	network := &net.IPNet{
		IP:   []byte{192, 168, 0, 1},
		Mask: []byte{255, 255, 255, 0},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 253; j++ {
			_, err := RequestIP(network, nil)
			if err != nil {
				b.Fatal(err)
			}
		}
		reset()
	}
}
