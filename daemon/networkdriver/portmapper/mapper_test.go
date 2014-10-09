package portmapper

import (
	"net"
	"testing"

	"github.com/docker/docker/daemon/networkdriver/portallocator"
	"github.com/docker/docker/pkg/iptables"
)

func init() {
	// override this func to mock out the proxy server
	NewProxy = NewMockProxyCommand
}

func reset() {
	chain = nil
	currentMappings = make(map[string]*mapping)
}

func TestSetIptablesChain(t *testing.T) {
	defer reset()

	c := &iptables.Chain{
		Name:   "TEST",
		Bridge: "192.168.1.1",
	}

	if chain != nil {
		t.Fatal("chain should be nil at init")
	}

	SetIptablesChain(c)
	if chain == nil {
		t.Fatal("chain should not be nil after set")
	}
}

func TestMapPorts(t *testing.T) {
	dstIp1 := net.ParseIP("192.168.0.1")
	dstIp2 := net.ParseIP("192.168.0.2")
	dstAddr1 := &net.TCPAddr{IP: dstIp1, Port: 80}
	dstAddr2 := &net.TCPAddr{IP: dstIp2, Port: 80}

	addrEqual := func(addr1, addr2 net.Addr) bool {
		return (addr1.Network() == addr2.Network()) && (addr1.String() == addr2.String())
	}

	if host, err := Map(net.ParseIP("172.16.0.1"), "tcp", 1080, dstIp1, 80); err != nil {
		t.Fatalf("Failed to allocate port: %s", err)
	} else if !addrEqual(dstAddr1, host[0]) {
		t.Fatalf("Incorrect mapping result: expected %s:%s, got %s:%s",
			dstAddr1.String(), dstAddr1.Network(), host[0].String(), host[0].Network())
	}

	if _, err := Map(net.ParseIP("172.16.0.1"), "tcp", 1080, dstIp1, 80); err == nil {
		t.Fatalf("Port is in use - mapping should have failed")
	}

	if _, err := Map(net.ParseIP("172.16.0.2"), "tcp", 1080, dstIp1, 80); err == nil {
		t.Fatalf("Port is in use - mapping should have failed")
	}

	if _, err := Map(net.ParseIP("172.16.0.2"), "tcp", 1080, dstIp2, 80); err != nil {
		t.Fatalf("Failed to allocate port: %s", err)
	}

	if Unmap(dstAddr1) != nil {
		t.Fatalf("Failed to release port")
	}

	if Unmap(dstAddr2) != nil {
		t.Fatalf("Failed to release port")
	}

	if Unmap(dstAddr2) == nil {
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
	dstIp1 := net.ParseIP("0.0.0.0")

	hosts := []net.Addr{}
	var host []net.Addr
	var err error

	defer func() {
		for _, val := range hosts {
			Unmap(val)
		}
	}()

	for i := 0; i < 10; i++ {
		for i := portallocator.BeginPortRange; i < portallocator.EndPortRange; i++ {
			if host, err = Map(net.ParseIP("172.16.0.1"), "tcp", 1080, dstIp1, 0); err != nil {
				t.Fatal(err)
			}

			hosts = append(hosts, host[0])
		}

		if _, err := Map(net.ParseIP("172.16.0.1"), "tcp", 1080, dstIp1, portallocator.BeginPortRange); err == nil {
			t.Fatalf("Port %d should be bound but is not", portallocator.BeginPortRange)
		}

		for _, val := range hosts {
			if err := Unmap(val); err != nil {
				t.Fatal(err)
			}
		}

		hosts = []net.Addr{}
	}
}

func TestMapTCPUDPPorts(t *testing.T) {
	dstIp1 := net.ParseIP("0.0.0.0")

	hosts := []net.Addr{}
	var host []net.Addr
	var err error

	defer func() {
		for _, val := range hosts {
			Unmap(val)
		}
	}()

	for i := 0; i < 10; i++ {
		for i := portallocator.BeginPortRange; i < portallocator.BeginPortRange+5; i++ {
			if host, err = Map(net.ParseIP("172.16.0.1"), "tcpudp", 1080, dstIp1, 0); err != nil {
				t.Fatal(err)
			}

			hosts = append(hosts, host[0])
		}

		if _, err := Map(net.ParseIP("172.16.0.1"), "tcpudp", 1080, dstIp1, portallocator.BeginPortRange); err == nil {
			t.Fatalf("Port %d should be bound but is not", portallocator.BeginPortRange)
		}

		for _, val := range hosts {
			if err := Unmap(val); err != nil {
				t.Fatal(err)
			}
		}

		hosts = []net.Addr{}
	}
}
