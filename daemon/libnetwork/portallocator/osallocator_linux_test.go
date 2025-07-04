package portallocator

import (
	"io"
	"net"
	"os"
	"testing"

	"github.com/docker/docker/daemon/libnetwork/netutils"
	"github.com/docker/docker/daemon/libnetwork/types"
	"github.com/ishidawataru/sctp"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func listen(t *testing.T, proto types.Protocol, addr net.IP, port int) io.Closer {
	var l io.Closer
	var err error

	switch proto {
	case types.TCP:
		l, err = net.ListenTCP("tcp", &net.TCPAddr{IP: addr, Port: port})
	case types.UDP:
		l, err = net.ListenUDP("udp", &net.UDPAddr{IP: addr, Port: port})
	case types.SCTP:
		l, err = sctp.ListenSCTP("sctp", &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: addr}}, Port: port})
	default:
		t.Fatalf("protocol %s not supported", proto)
	}

	assert.NilError(t, err)
	return l
}

func closeSocks(t *testing.T, files []*os.File) {
	for _, f := range files {
		if f != nil {
			err := f.Close()
			assert.NilError(t, err)
		}
	}
}

func TestAllocateExactPort(t *testing.T) {
	alloc := NewOSAllocator()
	addrs := []net.IP{net.IPv4zero}

	for _, expProto := range []types.Protocol{types.TCP, types.UDP, types.SCTP} {
		t.Run(expProto.String(), func(t *testing.T) {
			port, socks, err := alloc.RequestPortsInRange(addrs, expProto, 31234, 31234)
			defer alloc.ReleasePorts(addrs, expProto, port)
			defer closeSocks(t, socks)

			assert.NilError(t, err)
			assert.Equal(t, port, 31234)
			assert.Check(t, is.Len(socks, 1))
		})
	}
}

func TestAllocateExactPortForMultipleAddrs(t *testing.T) {
	alloc := NewOSAllocator()

	addrs := []net.IP{
		net.ParseIP("127.0.0.1"),
		net.ParseIP("127.0.0.2"),
		net.ParseIP("127.0.0.3"),
	}
	if netutils.IsV6Listenable() {
		addrs = append(addrs, net.IPv6loopback)
	}

	for _, expProto := range []types.Protocol{types.TCP, types.UDP, types.SCTP} {
		t.Run(expProto.String(), func(t *testing.T) {
			port, socks, err := alloc.RequestPortsInRange(addrs, expProto, 31234, 31234)
			defer alloc.ReleasePorts(addrs, expProto, port)
			defer closeSocks(t, socks)

			assert.NilError(t, err)
			assert.Equal(t, port, 31234)
			assert.Check(t, is.Len(socks, len(addrs)))

			for i, sock := range socks {
				sa, err := unix.Getsockname(int(sock.Fd()))
				assert.NilError(t, err)

				expAddr := addrs[i]
				if expAddr.To4() != nil {
					assert.Check(t, is.Equal(sa.(*unix.SockaddrInet4).Port, port))
					addr := net.IP(sa.(*unix.SockaddrInet4).Addr[:])
					assert.Check(t, addr.Equal(expAddr))
				} else {
					assert.Check(t, is.Equal(sa.(*unix.SockaddrInet6).Port, port))
					addr := net.IP(sa.(*unix.SockaddrInet6).Addr[:])
					assert.Check(t, addr.Equal(expAddr))
				}

				proto, err := unix.GetsockoptInt(int(sock.Fd()), unix.SOL_SOCKET, unix.SO_PROTOCOL)
				assert.NilError(t, err)
				assert.Check(t, is.Equal(proto, int(expProto)))
			}
		})
	}
}

func TestAllocateExactPortInUse(t *testing.T) {
	alloc := NewOSAllocator()
	addrs := []net.IP{net.ParseIP("127.0.0.1")}

	for _, tc := range []struct {
		proto  types.Protocol
		expErr string
	}{
		{proto: types.TCP, expErr: "failed to bind host port 127.0.0.1:12345/tcp: address already in use"},
		{proto: types.UDP, expErr: "failed to bind host port 127.0.0.1:12345/udp: address already in use"},
		{proto: types.SCTP, expErr: "failed to bind host port 127.0.0.1:12345/sctp: address already in use"},
	} {
		t.Run(tc.proto.String(), func(t *testing.T) {
			l := listen(t, tc.proto, net.IPv4zero, 12345)
			defer l.Close()

			// Port 12345 is in use, so the first allocation attempt should fail
			_, _, err := alloc.RequestPortsInRange(addrs, tc.proto, 12345, 12345)
			assert.ErrorContains(t, err, tc.expErr)

			// Close port 12345, and retry the allocation — it should succeed this time
			err = l.Close()
			assert.NilError(t, err)

			port, socks, err := alloc.RequestPortsInRange(addrs, tc.proto, 12345, 12345)
			defer alloc.ReleasePorts(addrs, tc.proto, port)
			defer closeSocks(t, socks)

			assert.NilError(t, err)
			assert.Equal(t, port, 12345)
		})
	}
}

func TestAllocateRangePortInUse(t *testing.T) {
	alloc := NewOSAllocator()
	addrs := []net.IP{net.ParseIP("127.0.0.1")}

	for _, tc := range []struct {
		proto  types.Protocol
		expErr string
	}{
		{proto: types.TCP, expErr: "failed to bind host port 127.0.0.1:8080/tcp: address already in use"},
		{proto: types.UDP, expErr: "failed to bind host port 127.0.0.1:8080/udp: address already in use"},
		{proto: types.SCTP, expErr: "failed to bind host port 127.0.0.1:8080/sctp: address already in use"},
	} {
		t.Run(tc.proto.String(), func(t *testing.T) {
			l := listen(t, tc.proto, net.IPv4zero, 8080)
			defer l.Close()

			// Port 8080 is in use, so the first allocation attempt should fail
			_, _, err := alloc.RequestPortsInRange(addrs, tc.proto, 8080, 8081)
			assert.ErrorContains(t, err, tc.expErr)

			// Retry allocation with same range — this time, it should pick 8081 successfully
			port, socks81, err := alloc.RequestPortsInRange(addrs, tc.proto, 8080, 8081)
			defer alloc.ReleasePorts(addrs, tc.proto, port)
			defer closeSocks(t, socks81)

			assert.NilError(t, err)
			assert.Equal(t, port, 8081)

			// Close port 8080, and try to allocate the same port again — it should succeed this time
			err = l.Close()
			assert.NilError(t, err)

			port, socks80, err := alloc.RequestPortsInRange(addrs, tc.proto, 8080, 8081)
			defer alloc.ReleasePorts(addrs, tc.proto, port)
			defer closeSocks(t, socks80)

			assert.NilError(t, err)
			assert.Equal(t, port, 8080)
		})
	}
}

func TestRepeatAllocation(t *testing.T) {
	alloc := NewOSAllocator()
	addrs := []net.IP{net.ParseIP("127.0.0.1")}

	for _, proto := range []types.Protocol{types.TCP, types.UDP, types.SCTP} {
		t.Run(proto.String(), func(t *testing.T) {
			// First allocation
			port, socks, err := alloc.RequestPortsInRange(addrs, proto, 8080, 8080)
			defer alloc.ReleasePorts(addrs, proto, port)
			defer func() {
				closeSocks(t, socks)
			}()

			assert.NilError(t, err)
			assert.Equal(t, port, 8080)

			// Release the port
			alloc.ReleasePorts(addrs, proto, port)
			closeSocks(t, socks)

			// Repeat the same allocation
			port, socks, err = alloc.RequestPortsInRange(addrs, proto, 8080, 8080)
			assert.NilError(t, err)
			assert.Equal(t, port, 8080)
		})
	}
}
