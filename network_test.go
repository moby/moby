package docker

import (
	"github.com/dotcloud/docker/pkg/iptables"
	"github.com/dotcloud/docker/proxy"
	"net"
	"testing"
)

func TestPortAllocation(t *testing.T) {
	ip := net.ParseIP("192.168.0.1")
	ip2 := net.ParseIP("192.168.0.2")
	allocator, err := newPortAllocator()
	if err != nil {
		t.Fatal(err)
	}
	if port, err := allocator.Acquire(ip, 80); err != nil {
		t.Fatal(err)
	} else if port != 80 {
		t.Fatalf("Acquire(80) should return 80, not %d", port)
	}
	port, err := allocator.Acquire(ip, 0)
	if err != nil {
		t.Fatal(err)
	}
	if port <= 0 {
		t.Fatalf("Acquire(0) should return a non-zero port")
	}
	if _, err := allocator.Acquire(ip, port); err == nil {
		t.Fatalf("Acquiring a port already in use should return an error")
	}
	if newPort, err := allocator.Acquire(ip, 0); err != nil {
		t.Fatal(err)
	} else if newPort == port {
		t.Fatalf("Acquire(0) allocated the same port twice: %d", port)
	}
	if _, err := allocator.Acquire(ip, 80); err == nil {
		t.Fatalf("Acquiring a port already in use should return an error")
	}
	if _, err := allocator.Acquire(ip2, 80); err != nil {
		t.Fatalf("It should be possible to allocate the same port on a different interface")
	}
	if _, err := allocator.Acquire(ip2, 80); err == nil {
		t.Fatalf("Acquiring a port already in use should return an error")
	}
	if err := allocator.Release(ip, 80); err != nil {
		t.Fatal(err)
	}
	if _, err := allocator.Acquire(ip, 80); err != nil {
		t.Fatal(err)
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
	conv := intToIP(i)
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

func AssertOverlap(CIDRx string, CIDRy string, t *testing.T) {
	_, netX, _ := net.ParseCIDR(CIDRx)
	_, netY, _ := net.ParseCIDR(CIDRy)
	if !networkOverlaps(netX, netY) {
		t.Errorf("%v and %v should overlap", netX, netY)
	}
}

func AssertNoOverlap(CIDRx string, CIDRy string, t *testing.T) {
	_, netX, _ := net.ParseCIDR(CIDRx)
	_, netY, _ := net.ParseCIDR(CIDRy)
	if networkOverlaps(netX, netY) {
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
	AssertOverlap("172.16.1.1/24", "172.16.0.1/23", t)
	//netY starts and ends before netX
	AssertNoOverlap("172.16.1.1/25", "172.16.0.1/24", t)
	//netX starts and ends before netY
	AssertNoOverlap("172.16.1.1/25", "172.16.2.1/24", t)
}

func TestCheckRouteOverlaps(t *testing.T) {
	routesData := []string{"10.0.2.0/32", "10.0.3.0/24", "10.0.42.0/24", "172.16.42.0/24", "192.168.142.0/24"}

	routes := []*net.IPNet{}
	for _, addr := range routesData {
		_, netX, _ := net.ParseCIDR(addr)
		routes = append(routes, netX)
	}

	_, netX, _ := net.ParseCIDR("172.16.0.1/24")
	if err := checkRouteOverlaps(routes, netX); err != nil {
		t.Fatal(err)
	}

	_, netX, _ = net.ParseCIDR("10.0.2.0/24")
	if err := checkRouteOverlaps(routes, netX); err == nil {
		t.Fatalf("10.0.2.0/24 and 10.0.2.0 should overlap but it doesn't")
	}
}

func TestCheckNameserverOverlaps(t *testing.T) {
	nameservers := []string{"10.0.2.3/32", "192.168.102.1/32"}

	_, netX, _ := net.ParseCIDR("10.0.2.3/32")

	if err := checkNameserverOverlaps(nameservers, netX); err == nil {
		t.Fatalf("%s should overlap 10.0.2.3/32 but doesn't", netX)
	}

	_, netX, _ = net.ParseCIDR("192.168.102.2/32")

	if err := checkNameserverOverlaps(nameservers, netX); err != nil {
		t.Fatalf("%s should not overlap %v but it does", netX, nameservers)
	}
}

type StubProxy struct {
	frontendAddr *net.Addr
	backendAddr  *net.Addr
}

func (proxy *StubProxy) Run()                   {}
func (proxy *StubProxy) Close()                 {}
func (proxy *StubProxy) FrontendAddr() net.Addr { return *proxy.frontendAddr }
func (proxy *StubProxy) BackendAddr() net.Addr  { return *proxy.backendAddr }

func NewStubProxy(frontendAddr, backendAddr net.Addr) (proxy.Proxy, error) {
	return &StubProxy{
		frontendAddr: &frontendAddr,
		backendAddr:  &backendAddr,
	}, nil
}

func TestPortMapper(t *testing.T) {
	// FIXME: is this iptables chain still used anywhere?
	var chain *iptables.Chain
	mapper := &PortMapper{
		tcpMapping:       make(map[string]*net.TCPAddr),
		tcpProxies:       make(map[string]proxy.Proxy),
		udpMapping:       make(map[string]*net.UDPAddr),
		udpProxies:       make(map[string]proxy.Proxy),
		iptables:         chain,
		defaultIp:        net.IP("0.0.0.0"),
		proxyFactoryFunc: NewStubProxy,
	}

	dstIp1 := net.ParseIP("192.168.0.1")
	dstIp2 := net.ParseIP("192.168.0.2")
	srcAddr1 := &net.TCPAddr{Port: 1080, IP: net.ParseIP("172.16.0.1")}
	srcAddr2 := &net.TCPAddr{Port: 1080, IP: net.ParseIP("172.16.0.2")}

	if err := mapper.Map(dstIp1, 80, srcAddr1); err != nil {
		t.Fatalf("Failed to allocate port: %s", err)
	}

	if mapper.Map(dstIp1, 80, srcAddr1) == nil {
		t.Fatalf("Port is in use - mapping should have failed")
	}

	if mapper.Map(dstIp1, 80, srcAddr2) == nil {
		t.Fatalf("Port is in use - mapping should have failed")
	}

	if err := mapper.Map(dstIp2, 80, srcAddr2); err != nil {
		t.Fatalf("Failed to allocate port: %s", err)
	}

	if mapper.Unmap(dstIp1, 80, "tcp") != nil {
		t.Fatalf("Failed to release port")
	}

	if mapper.Unmap(dstIp2, 80, "tcp") != nil {
		t.Fatalf("Failed to release port")
	}

	if mapper.Unmap(dstIp2, 80, "tcp") == nil {
		t.Fatalf("Port already released, but no error reported")
	}
}
