package portallocator

import (
	"errors"
	"net"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestRequestNewPort(t *testing.T) {
	p := newInstance()

	port, err := p.RequestPort(net.IPv4zero, "tcp", 0)
	if err != nil {
		t.Fatal(err)
	}

	if expected := p.begin; port != expected {
		t.Fatalf("Expected port %d got %d", expected, port)
	}
}

func TestRequestSpecificPort(t *testing.T) {
	p := newInstance()

	port, err := p.RequestPort(net.IPv4zero, "tcp", 5000)
	if err != nil {
		t.Fatal(err)
	}

	if port != 5000 {
		t.Fatalf("Expected port 5000 got %d", port)
	}
}

func TestReleasePort(t *testing.T) {
	p := newInstance()

	port, err := p.RequestPort(net.IPv4zero, "tcp", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if port != 5000 {
		t.Fatalf("Expected port 5000 got %d", port)
	}

	p.ReleasePort(net.IPv4zero, "tcp", 5000)
}

func TestReuseReleasedPort(t *testing.T) {
	p := newInstance()

	port, err := p.RequestPort(net.IPv4zero, "tcp", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if port != 5000 {
		t.Fatalf("Expected port 5000 got %d", port)
	}

	p.ReleasePort(net.IPv4zero, "tcp", 5000)

	port, err = p.RequestPort(net.IPv4zero, "tcp", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if port != 5000 {
		t.Fatalf("Expected port 5000 got %d", port)
	}
}

func TestReleaseUnreadledPort(t *testing.T) {
	p := newInstance()

	port, err := p.RequestPort(net.IPv4zero, "tcp", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if port != 5000 {
		t.Fatalf("Expected port 5000 got %d", port)
	}

	_, err = p.RequestPort(net.IPv4zero, "tcp", 5000)

	var expectedErrType alreadyAllocatedErr
	if !errors.As(err, &expectedErrType) {
		t.Fatalf("Expected port allocation error got %s", err)
	}
}

func TestUnknowProtocol(t *testing.T) {
	p := newInstance()

	if _, err := p.RequestPort(net.IPv4zero, "tcpp", 0); err != errUnknownProtocol {
		t.Fatalf("Expected error %s got %s", errUnknownProtocol, err)
	}
}

func TestAllocateAllPorts(t *testing.T) {
	p := newInstance()

	for i := 0; i <= p.end-p.begin; i++ {
		port, err := p.RequestPort(net.IPv4zero, "tcp", 0)
		if err != nil {
			t.Fatal(err)
		}

		if expected := p.begin + i; port != expected {
			t.Fatalf("Expected port %d got %d", expected, port)
		}
	}

	if _, err := p.RequestPort(net.IPv4zero, "tcp", 0); err != errAllPortsAllocated {
		t.Fatalf("Expected error %s got %s", errAllPortsAllocated, err)
	}

	_, err := p.RequestPort(net.IPv4zero, "udp", 0)
	if err != nil {
		t.Fatal(err)
	}

	// release a port in the middle and ensure we get another tcp port
	port := p.begin + 5
	p.ReleasePort(net.IPv4zero, "tcp", port)
	newPort, err := p.RequestPort(net.IPv4zero, "tcp", 0)
	if err != nil {
		t.Fatal(err)
	}
	if newPort != port {
		t.Fatalf("Expected port %d got %d", port, newPort)
	}

	// now pm.last == newPort, release it so that it's the only free port of
	// the range, and ensure we get it back
	p.ReleasePort(net.IPv4zero, "tcp", newPort)
	port, err = p.RequestPort(net.IPv4zero, "tcp", 0)
	if err != nil {
		t.Fatal(err)
	}
	if newPort != port {
		t.Fatalf("Expected port %d got %d", newPort, port)
	}
}

func BenchmarkAllocatePorts(b *testing.B) {
	p := newInstance()

	for n := 0; n < b.N; n++ {
		for i := 0; i <= p.end-p.begin; i++ {
			port, err := p.RequestPort(net.IPv4zero, "tcp", 0)
			if err != nil {
				b.Fatal(err)
			}

			if expected := p.begin + i; port != expected {
				b.Fatalf("Expected port %d got %d", expected, port)
			}
		}
		p.ReleaseAll()
	}
}

func TestPortAllocation(t *testing.T) {
	p := newInstance()

	ip := net.ParseIP("192.168.0.1")
	ip2 := net.ParseIP("192.168.0.2")
	if port, err := p.RequestPort(ip, "tcp", 80); err != nil {
		t.Fatal(err)
	} else if port != 80 {
		t.Fatalf("Acquire(80) should return 80, not %d", port)
	}
	port, err := p.RequestPort(ip, "tcp", 0)
	if err != nil {
		t.Fatal(err)
	}
	if port <= 0 {
		t.Fatalf("Acquire(0) should return a non-zero port")
	}

	if _, err := p.RequestPort(ip, "tcp", port); err == nil {
		t.Fatalf("Acquiring a port already in use should return an error")
	}

	if newPort, err := p.RequestPort(ip, "tcp", 0); err != nil {
		t.Fatal(err)
	} else if newPort == port {
		t.Fatalf("Acquire(0) allocated the same port twice: %d", port)
	}

	if _, err := p.RequestPort(ip, "tcp", 80); err == nil {
		t.Fatalf("Acquiring a port already in use should return an error")
	}
	if _, err := p.RequestPort(ip2, "tcp", 80); err != nil {
		t.Fatalf("It should be possible to allocate the same port on a different interface")
	}
	if _, err := p.RequestPort(ip2, "tcp", 80); err == nil {
		t.Fatalf("Acquiring a port already in use should return an error")
	}
	p.ReleasePort(ip, "tcp", 80)
	if _, err := p.RequestPort(ip, "tcp", 80); err != nil {
		t.Fatal(err)
	}

	port, err = p.RequestPort(ip, "tcp", 0)
	if err != nil {
		t.Fatal(err)
	}
	port2, err := p.RequestPort(ip, "tcp", port+1)
	if err != nil {
		t.Fatal(err)
	}
	port3, err := p.RequestPort(ip, "tcp", 0)
	if err != nil {
		t.Fatal(err)
	}
	if port3 == port2 {
		t.Fatal("Requesting a dynamic port should never allocate a used port")
	}
}

func TestPortAllocationWithCustomRange(t *testing.T) {
	p := newInstance()

	start, end := 8081, 8082
	specificPort := 8000

	// get an ephemeral port.
	port1, err := p.RequestPortInRange(net.IPv4zero, "tcp", 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	// request invalid ranges
	if _, err := p.RequestPortInRange(net.IPv4zero, "tcp", 0, end); err == nil {
		t.Fatalf("Expected error for invalid range %d-%d", 0, end)
	}
	if _, err := p.RequestPortInRange(net.IPv4zero, "tcp", start, 0); err == nil {
		t.Fatalf("Expected error for invalid range %d-%d", 0, end)
	}
	if _, err := p.RequestPortInRange(net.IPv4zero, "tcp", 8081, 8080); err == nil {
		t.Fatalf("Expected error for invalid range %d-%d", 0, end)
	}

	// request a single port
	port, err := p.RequestPortInRange(net.IPv4zero, "tcp", specificPort, specificPort)
	if err != nil {
		t.Fatal(err)
	}
	if port != specificPort {
		t.Fatalf("Expected port %d, got %d", specificPort, port)
	}

	// get a port from the range
	port2, err := p.RequestPortInRange(net.IPv4zero, "tcp", start, end)
	if err != nil {
		t.Fatal(err)
	}
	if port2 < start || port2 > end {
		t.Fatalf("Expected a port between %d and %d, got %d", start, end, port2)
	}
	// get another ephemeral port (should be > port1)
	port3, err := p.RequestPortInRange(net.IPv4zero, "tcp", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if port3 < port1 {
		t.Fatalf("Expected new port > %d in the ephemeral range, got %d", port1, port3)
	}
	// get another (and in this case the only other) port from the range
	port4, err := p.RequestPortInRange(net.IPv4zero, "tcp", start, end)
	if err != nil {
		t.Fatal(err)
	}
	if port4 < start || port4 > end {
		t.Fatalf("Expected a port between %d and %d, got %d", start, end, port4)
	}
	if port4 == port2 {
		t.Fatal("Allocated the same port from a custom range")
	}
	// request 3rd port from the range of 2
	if _, err := p.RequestPortInRange(net.IPv4zero, "tcp", start, end); err != errAllPortsAllocated {
		t.Fatalf("Expected error %s got %s", errAllPortsAllocated, err)
	}
}

func TestNoDuplicateBPR(t *testing.T) {
	p := newInstance()

	if port, err := p.RequestPort(net.IPv4zero, "tcp", p.begin); err != nil {
		t.Fatal(err)
	} else if port != p.begin {
		t.Fatalf("Expected port %d got %d", p.begin, port)
	}

	if port, err := p.RequestPort(net.IPv4zero, "tcp", 0); err != nil {
		t.Fatal(err)
	} else if port == p.begin {
		t.Fatalf("Acquire(0) allocated the same port twice: %d", port)
	}
}

func TestRequestPortForMultipleIPs(t *testing.T) {
	p := newInstance()

	addrs := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::")}

	// Default port range.
	port, err := p.RequestPortsInRange(addrs, "tcp", 0, 0)
	assert.Check(t, err)
	assert.Check(t, is.Equal(port, p.begin))

	// Single-port range.
	port, err = p.RequestPortsInRange(addrs, "tcp", 10000, 10000)
	assert.Check(t, err)
	assert.Check(t, is.Equal(port, 10000))

	// Same single-port range, expect an error.
	_, err = p.RequestPortsInRange(addrs, "tcp", 10000, 10000)
	assert.Check(t, is.Error(err, "Bind for 127.0.0.1:10000 failed: port is already allocated"))

	// Release the port from one address.
	p.ReleasePort(addrs[0], "tcp", 10000)

	// Same single-port range, still expect an error because the port's still held
	// for the second address.
	_, err = p.RequestPortsInRange(addrs, "tcp", 10000, 10000)
	assert.Check(t, is.Error(err, "Bind for :::10000 failed: port is already allocated"))

	// Release the port from the other address.
	p.ReleasePort(addrs[1], "tcp", 10000)

	// Should now be able to re-allocate the port.
	port, err = p.RequestPortsInRange(addrs, "tcp", 10000, 10000)
	assert.Check(t, err)
	assert.Check(t, is.Equal(port, 10000))

	// Multi-port range.
	for i := 20000; i < 20004; i += 1 {
		port, err = p.RequestPortsInRange(addrs, "tcp", 20000, 20004)
		assert.Check(t, err)
		assert.Check(t, is.Equal(port, i))
	}
}
