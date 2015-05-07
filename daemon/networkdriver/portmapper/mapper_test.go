package portmapper

import (
	"net"
	"testing"

	"github.com/docker/docker/pkg/iptables"
)

func init() {
	// override this func to mock out the proxy server
	NewProxy = NewMockProxyCommand
}

func TestSetIptablesChain(t *testing.T) {
	pm := New()

	c := &iptables.Chain{
		Name:   "TEST",
		Bridge: "192.168.1.1",
	}

	if pm.chain != nil {
		t.Fatal("chain should be nil at init")
	}

	pm.SetIptablesChain(c)
	if pm.chain == nil {
		t.Fatal("chain should not be nil after set")
	}
}

func TestMapPorts(t *testing.T) {
	pm := New()
	dstIp1 := net.ParseIP("192.168.0.1")
	dstIp2 := net.ParseIP("192.168.0.2")
	dstAddr1 := &net.TCPAddr{IP: dstIp1, Port: 80}
	dstAddr2 := &net.TCPAddr{IP: dstIp2, Port: 80}

	srcAddr1 := &net.TCPAddr{Port: 1080, IP: net.ParseIP("172.16.0.1")}
	srcAddr2 := &net.TCPAddr{Port: 1080, IP: net.ParseIP("172.16.0.2")}

	addrEqual := func(addr1, addr2 net.Addr) bool {
		return (addr1.Network() == addr2.Network()) && (addr1.String() == addr2.String())
	}

	if host, err := pm.Map(srcAddr1, dstIp1, 80, true); err != nil {
		t.Fatalf("Failed to allocate port: %s", err)
	} else if !addrEqual(dstAddr1, host) {
		t.Fatalf("Incorrect mapping result: expected %s:%s, got %s:%s",
			dstAddr1.String(), dstAddr1.Network(), host.String(), host.Network())
	}

	if _, err := pm.Map(srcAddr1, dstIp1, 80, true); err == nil {
		t.Fatalf("Port is in use - mapping should have failed")
	}

	if _, err := pm.Map(srcAddr2, dstIp1, 80, true); err == nil {
		t.Fatalf("Port is in use - mapping should have failed")
	}

	if _, err := pm.Map(srcAddr2, dstIp2, 80, true); err != nil {
		t.Fatalf("Failed to allocate port: %s", err)
	}

	if pm.Unmap(dstAddr1) != nil {
		t.Fatalf("Failed to release port")
	}

	if pm.Unmap(dstAddr2) != nil {
		t.Fatalf("Failed to release port")
	}

	if pm.Unmap(dstAddr2) == nil {
		t.Fatalf("Port already released, but no error reported")
	}
}

func TestGetUDPKey(t *testing.T) {
	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.5"), Port: 53}

	key := getKey(addr)

	if expected := "192.168.1.5:53/udp"; key != expected {
		t.Fatalf("expected key %s got %s", expected, key)
	}
}

func TestGetTCPKey(t *testing.T) {
	addr := &net.TCPAddr{IP: net.ParseIP("192.168.1.5"), Port: 80}

	key := getKey(addr)

	if expected := "192.168.1.5:80/tcp"; key != expected {
		t.Fatalf("expected key %s got %s", expected, key)
	}
}

func TestGetUDPIPAndPort(t *testing.T) {
	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.5"), Port: 53}

	ip, port := getIPAndPort(addr)
	if expected := "192.168.1.5"; ip.String() != expected {
		t.Fatalf("expected ip %s got %s", expected, ip)
	}

	if ep := 53; port != ep {
		t.Fatalf("expected port %d got %d", ep, port)
	}
}

func TestMapAllPortsSingleInterface(t *testing.T) {
	pm := New()
	dstIp1 := net.ParseIP("0.0.0.0")
	srcAddr1 := &net.TCPAddr{Port: 1080, IP: net.ParseIP("172.16.0.1")}

	hosts := []net.Addr{}
	var host net.Addr
	var err error

	defer func() {
		for _, val := range hosts {
			pm.Unmap(val)
		}
	}()

	for i := 0; i < 10; i++ {
		start, end := pm.Allocator.Begin, pm.Allocator.End
		for i := start; i < end; i++ {
			if host, err = pm.Map(srcAddr1, dstIp1, 0, true); err != nil {
				t.Fatal(err)
			}

			hosts = append(hosts, host)
		}

		if _, err := pm.Map(srcAddr1, dstIp1, start, true); err == nil {
			t.Fatalf("Port %d should be bound but is not", start)
		}

		for _, val := range hosts {
			if err := pm.Unmap(val); err != nil {
				t.Fatal(err)
			}
		}

		hosts = []net.Addr{}
	}
}
