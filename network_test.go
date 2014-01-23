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
