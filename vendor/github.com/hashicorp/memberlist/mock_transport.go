package memberlist

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

// MockNetwork is used as a factory that produces MockTransport instances which
// are uniquely addressed and wired up to talk to each other.
type MockNetwork struct {
	transports map[string]*MockTransport
	port       int
}

// NewTransport returns a new MockTransport with a unique address, wired up to
// talk to the other transports in the MockNetwork.
func (n *MockNetwork) NewTransport() *MockTransport {
	n.port += 1
	addr := fmt.Sprintf("127.0.0.1:%d", n.port)
	transport := &MockTransport{
		net:      n,
		addr:     &MockAddress{addr},
		packetCh: make(chan *Packet),
		streamCh: make(chan net.Conn),
	}

	if n.transports == nil {
		n.transports = make(map[string]*MockTransport)
	}
	n.transports[addr] = transport
	return transport
}

// MockAddress is a wrapper which adds the net.Addr interface to our mock
// address scheme.
type MockAddress struct {
	addr string
}

// See net.Addr.
func (a *MockAddress) Network() string {
	return "mock"
}

// See net.Addr.
func (a *MockAddress) String() string {
	return a.addr
}

// MockTransport directly plumbs messages to other transports its MockNetwork.
type MockTransport struct {
	net      *MockNetwork
	addr     *MockAddress
	packetCh chan *Packet
	streamCh chan net.Conn
}

// See Transport.
func (t *MockTransport) FinalAdvertiseAddr(string, int) (net.IP, int, error) {
	host, portStr, err := net.SplitHostPort(t.addr.String())
	if err != nil {
		return nil, 0, err
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return nil, 0, fmt.Errorf("Failed to parse IP %q", host)
	}

	port, err := strconv.ParseInt(portStr, 10, 16)
	if err != nil {
		return nil, 0, err
	}

	return ip, int(port), nil
}

// See Transport.
func (t *MockTransport) WriteTo(b []byte, addr string) (time.Time, error) {
	dest, ok := t.net.transports[addr]
	if !ok {
		return time.Time{}, fmt.Errorf("No route to %q", addr)
	}

	now := time.Now()
	dest.packetCh <- &Packet{
		Buf:       b,
		From:      t.addr,
		Timestamp: now,
	}
	return now, nil
}

// See Transport.
func (t *MockTransport) PacketCh() <-chan *Packet {
	return t.packetCh
}

// See Transport.
func (t *MockTransport) DialTimeout(addr string, timeout time.Duration) (net.Conn, error) {
	dest, ok := t.net.transports[addr]
	if !ok {
		return nil, fmt.Errorf("No route to %q", addr)
	}

	p1, p2 := net.Pipe()
	dest.streamCh <- p1
	return p2, nil
}

// See Transport.
func (t *MockTransport) StreamCh() <-chan net.Conn {
	return t.streamCh
}

// See Transport.
func (t *MockTransport) Shutdown() error {
	return nil
}
