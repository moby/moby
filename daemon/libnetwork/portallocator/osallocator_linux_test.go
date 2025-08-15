package portallocator

import (
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/ishidawataru/sctp"
	"github.com/moby/moby/v2/daemon/libnetwork/netutils"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
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

	for _, proto := range []types.Protocol{types.TCP, types.UDP, types.SCTP} {
		t.Run(proto.String(), func(t *testing.T) {
			l := listen(t, proto, net.IPv4zero, 8080)
			defer l.Close()

			// Port 8080 is in use, so it should pick 8081.
			port, socks81, err := alloc.RequestPortsInRange(addrs, proto, 8080, 8081)
			defer alloc.ReleasePorts(addrs, proto, port)
			defer closeSocks(t, socks81)

			assert.NilError(t, err)
			assert.Equal(t, port, 8081)

			// Try to allocate from that same range again — it should fail because 8080 is still in use.
			_, _, err = alloc.RequestPortsInRange(addrs, proto, 8080, 8081)
			assert.ErrorContains(t, err, fmt.Sprintf("failed to bind host port 127.0.0.1:8080/%s: address already in use", proto))

			// Close port 8080, and try to allocate from that range again — it should succeed this time.
			err = l.Close()
			assert.NilError(t, err)

			port, socks80, err := alloc.RequestPortsInRange(addrs, proto, 8080, 8081)
			defer alloc.ReleasePorts(addrs, proto, port)
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

func TestOnlyOneSocketBindsUDPPort(t *testing.T) {
	const port = 5000
	addr := netip.MustParseAddr("127.0.0.1")

	// Simulate another process binding to the same UDP port.
	f, err := bindTCPOrUDP(netip.AddrPortFrom(addr, port), syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
	assert.NilError(t, err)
	defer f.Close()

	// Then try to allocate that port using the OSAllocator.
	alloc := OSAllocator{allocator: newInstance()}
	_, socks, err := alloc.RequestPortsInRange([]net.IP{addr.AsSlice()}, syscall.IPPROTO_UDP, port, port)
	// In case RequestPortsInRange succeeded, close the sockets to not affect subsequent tests
	defer closeSocks(t, socks)

	assert.ErrorContains(t, err, "failed to bind host port")
	assert.Equal(t, len(socks), 0)
}

// TestSocketBacklogEqualsSomaxconn verifies that the listen syscall made for
// TCP / SCTP sockets has a backlog size equal to somaxconn.
func TestSocketBacklogEqualsSomaxconn(t *testing.T) {
	// Retrieve and parse sysctl net.core.somaxconn
	somaxconnSysctl, err := os.ReadFile("/proc/sys/net/core/somaxconn")
	assert.NilError(t, err)
	somaxconn, err := strconv.Atoi(strings.TrimSpace(string(somaxconnSysctl)))
	assert.NilError(t, err)

	// UDP isn't included in the list of protos to test because it doesn't have a backlog, and the ss Send-Q column
	// reports memory allocation instead of the socket's max backlog size (unlike TCP and SCTP).
	//
	// This is where the kernel writes the max backlog size into the sk struct: https://elixir.bootlin.com/linux/v6.16/source/net/ipv4/af_inet.c#L199
	//
	// And here's where the kernel writes the 'idiag_wqueue' field used by ss:
	//
	// - For TCP: https://elixir.bootlin.com/linux/v6.16/source/net/ipv4/tcp_diag.c#L25
	// - For UDP: https://elixir.bootlin.com/linux/v6.16/source/net/ipv4/udp_diag.c#L163
	// - For SCTP: https://elixir.bootlin.com/linux/v6.16/source/net/sctp/diag.c#L414
	for _, proto := range []types.Protocol{
		types.TCP,
		types.SCTP,
	} {
		t.Run(proto.String(), func(t *testing.T) {
			// Allocate an ephemeral port using the OSAllocator.
			alloc := NewOSAllocator()
			port, socks, err := alloc.RequestPortsInRange([]net.IP{net.IPv4zero}, proto, 0, 0)
			assert.NilError(t, err)
			defer closeSocks(t, socks)

			// 'ss' output looks like that:
			//
			//    Netid      State       Recv-Q      Send-Q           Local Address:Port            Peer Address:Port      Process
			//    tcp        LISTEN      0           4096                   0.0.0.0:32768                0.0.0.0:*
			//
			// The max backlog size ('idiag_wqueue' field of 'struct inet_diag_msg' in the kernel) is the 4th field in
			// the output.
			out, err := exec.Command("ss", "-Stl", "sport", "=", fmt.Sprintf("inet:%d", port)).Output()
			assert.NilError(t, err)

			t.Logf("ss output:\n" + string(out))

			lines := strings.Split(string(out), "\n")
			assert.Assert(t, len(lines) >= 2)

			fields := strings.Fields(lines[1])
			assert.Equal(t, len(fields), 6)

			backlog, err := strconv.Atoi(fields[3])
			assert.NilError(t, err)

			assert.Equal(t, fields[4], "0.0.0.0:"+strconv.Itoa(port))
			assert.Equal(t, backlog, somaxconn, "socket backlog should be equal to net.core.somaxconn")
		})
	}
}

// TestPacketsAreDroppedUntilDetachSocketFilter tests that SYN packets are
// dropped until DetachSocketFilter is called on the socket.
func TestPacketsAreDroppedUntilDetachSocketFilter(t *testing.T) {
	const port = 61100
	addr := net.ParseIP("127.0.0.1")

	var detached atomic.Bool
	dialCh, readCh := make(chan error), make(chan error)

	alloc := NewOSAllocator()
	_, socks, err := alloc.RequestPortsInRange([]net.IP{addr}, types.TCP, port, port)
	assert.NilError(t, err)
	assert.Check(t, len(socks) > 0)

	// Start a goroutine that attempts to connect to a listening socket. It'll send SYN packets until
	// DetachSocketFilter is called. If no filter is attached, the connection will succeed immediately, and it'll send
	// a payload of 0x0 (or the call to DetachSocketFilter will fail with an error). When the filter is detached, it'll
	// send a payload of 0x1, which will be read by the other goroutine.
	go func() {
		defer close(dialCh)

		c, err := net.Dial("tcp", net.JoinHostPort(addr.String(), strconv.Itoa(port)))
		if err != nil {
			dialCh <- fmt.Errorf("net.Dial: %w", err)
			return
		}
		defer c.Close()

		payload := []byte{0x0}
		if detached.Load() {
			payload = []byte{0x1}
		}

		n, err := c.Write(payload)
		if err != nil {
			dialCh <- fmt.Errorf("c.Write: %w", err)
			return
		}
		if n != len(payload) {
			dialCh <- fmt.Errorf("expected to write %d bytes, but wrote %d", len(payload), n)
		}
	}()

	// Start a goroutine that accepts a connection on the listening socket created by RequestPortsInRange, and reads
	// the payload sent by the 1st goroutine. It should not receive any new connection until DetachSocketFilter is
	// called on the socket.
	go func() {
		defer close(readCh)

		// net.FileListener dup's the fd, so DetachSocketFilter will have no effect. Use raw syscalls instead.
		sd := int(socks[0].Fd())

		var err error
		connfd, _, err := syscall.Accept(sd)
		if err != nil {
			readCh <- fmt.Errorf("syscall.Accept: %w", err)
			return
		}

		payload := make([]byte, 1)
		n, err := syscall.Read(connfd, payload)
		if err != nil {
			readCh <- fmt.Errorf("c.Read: %w", err)
			return
		}
		if n != 1 {
			readCh <- fmt.Errorf("expected to read 1 byte, but read %d", n)
			return
		}

		if payload[0] != 0x1 {
			readCh <- fmt.Errorf("expected payload 0x1, but got %x", payload[0])
		}
	}()

	// Sleep for a bit to make sure that both goroutines were scheduled.
	time.Sleep(500 * time.Millisecond)

	detached.Store(true)
	err = DetachSocketFilter(socks[0])
	assert.NilError(t, err)

	var dialStopped, readStopped bool
	for {
		if dialStopped && readStopped {
			return
		}

		select {
		case err, ok := <-dialCh:
			dialStopped = !ok
			assert.NilError(t, err)
		case err, ok := <-readCh:
			readStopped = !ok
			assert.NilError(t, err)
		}
	}
}
