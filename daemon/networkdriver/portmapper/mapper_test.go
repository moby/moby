package portmapper

import (
	"github.com/dotcloud/docker/pkg/iptables"
	"github.com/dotcloud/docker/pkg/proxy"
	"net"
	"testing"
)

func init() {
	// override this func to mock out the proxy server
	newProxy = proxy.NewStubProxy
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

	srcAddr1 := &net.TCPAddr{Port: 1080, IP: net.ParseIP("172.16.0.1")}
	srcAddr2 := &net.TCPAddr{Port: 1080, IP: net.ParseIP("172.16.0.2")}

	if err := Map(srcAddr1, dstIp1, 80); err != nil {
		t.Fatalf("Failed to allocate port: %s", err)
	}

	if Map(srcAddr1, dstIp1, 80) == nil {
		t.Fatalf("Port is in use - mapping should have failed")
	}

	if Map(srcAddr2, dstIp1, 80) == nil {
		t.Fatalf("Port is in use - mapping should have failed")
	}

	if err := Map(srcAddr2, dstIp2, 80); err != nil {
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
