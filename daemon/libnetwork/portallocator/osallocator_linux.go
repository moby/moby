package portallocator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"runtime"
	"syscall"

	"github.com/containerd/log"
	"github.com/ishidawataru/sctp"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
	"golang.org/x/net/bpf"
	"golang.org/x/sys/unix"
)

// maxAllocateAttempts is the maximum number of times OSAllocator.RequestPortsInRange
// will try to allocate a port before returning an error. This is an arbitrary
// limit.
const maxAllocateAttempts = 10

type OSAllocator struct {
	// allocator is used to logically reserve ports, to avoid those we know
	// are already in use. This is useful to ensure callers don't burn their
	// retry budget unnecessarily.
	allocator *PortAllocator
}

func NewOSAllocator() OSAllocator {
	return OSAllocator{
		allocator: Get(),
	}
}

// RequestPortsInRange reserves a port available in the range [portStart, portEnd]
// for all the specified addrs, and then try to bind/listen those addresses to
// allocate the port from the OS.
//
// It returns the allocated port, and all the sockets bound, or an error if the
// reserved port isn't available. These sockets have a filter set to ensure that
// the kernel doesn't accept connections on these. Callers must take care of
// calling DetachSocketFilter once they're ready to accept connections (e.g. after
// setting up DNAT rules, and before starting the userland proxy), and they must
// take care of closing the returned sockets.
//
// It's safe for concurrent use.
func (pa OSAllocator) RequestPortsInRange(addrs []net.IP, proto types.Protocol, portStart, portEnd int) (_ int, _ []*os.File, retErr error) {
	var port int
	var socks []*os.File
	var err error

	// Try up to maxAllocatePortAttempts times to get a port that's not already allocated.
	for i := range maxAllocateAttempts {
		port, socks, err = pa.attemptAllocation(addrs, proto, portStart, portEnd)
		if err == nil {
			break
		}
		// There is no point in immediately retrying to map an explicitly chosen port.
		if portStart != 0 && portStart == portEnd {
			log.G(context.TODO()).WithError(err).Warnf("Failed to allocate port")
			return 0, nil, err
		}
		// Do not retry if a port range is specified and all ports in that range are already allocated.
		if errors.Is(err, errAllPortsAllocated) {
			return 0, nil, err
		}
		log.G(context.TODO()).WithFields(log.Fields{
			"error":   err,
			"attempt": i + 1,
		}).Warn("Failed to allocate port")
	}

	if err != nil {
		// If the retry budget is exhausted and no free port could be found, return
		// the latest error.
		return 0, nil, err
	}

	return port, socks, nil
}

// attemptAllocation requests a port from the allocator and tries to bind/listen on that port
// on each of addrs. If the bind/listen fails, it means the allocator thought the port was free,
// but it was in use by some other process.
func (pa OSAllocator) attemptAllocation(addrs []net.IP, proto types.Protocol, portStart, portEnd int) (_ int, _ []*os.File, retErr error) {
	port, err := pa.allocator.RequestPortsInRange(addrs, proto.String(), portStart, portEnd)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if retErr != nil {
			for _, addr := range addrs {
				pa.allocator.ReleasePort(addr, proto.String(), port)
			}
		}
	}()

	var boundSocks []*os.File
	defer func() {
		if retErr != nil {
			for i, sock := range boundSocks {
				if err := sock.Close(); err != nil {
					log.G(context.TODO()).WithFields(log.Fields{
						"addr": addrs[i],
						"port": port,
					}).WithError(err).Warnf("failed to close socket during port allocation")
				}
			}
		}
	}()

	for _, addr := range addrs {
		addr, _ := netip.AddrFromSlice(addr)
		addrPort := netip.AddrPortFrom(addr.Unmap(), uint16(port))

		var sock *os.File
		var err error
		switch proto {
		case types.TCP:
			sock, err = listenTCP(addrPort)
		case types.UDP:
			sock, err = bindTCPOrUDP(addrPort, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
		case types.SCTP:
			sock, err = listenSCTP(addrPort)
		default:
			return 0, nil, fmt.Errorf("protocol %s not supported", proto)
		}

		if err != nil {
			return 0, nil, err
		}

		boundSocks = append(boundSocks, sock)
	}

	return port, boundSocks, nil
}

// ReleasePorts releases a common port reserved for a list of addrs. It doesn't
// close the sockets bound by [RequestPortsInRange]. This must be taken care of
// independently by the caller.
func (pa OSAllocator) ReleasePorts(addrs []net.IP, proto types.Protocol, port int) {
	for _, addr := range addrs {
		pa.allocator.ReleasePort(addr, proto.String(), port)
	}
}

func listenTCP(addr netip.AddrPort) (_ *os.File, retErr error) {
	boundSocket, err := bindTCPOrUDP(addr, syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
	if err != nil {
		return nil, err
	}

	somaxconn := -1 // silently capped to "/proc/sys/net/core/somaxconn"
	if err := syscall.Listen(int(boundSocket.Fd()), somaxconn); err != nil {
		return nil, fmt.Errorf("failed to listen on tcp socket: %w", err)
	}

	return boundSocket, nil
}

func bindTCPOrUDP(addr netip.AddrPort, typ int, proto types.Protocol) (_ *os.File, retErr error) {
	var domain int
	var sa syscall.Sockaddr
	if addr.Addr().Unmap().Is4() {
		domain = syscall.AF_INET
		sa = &syscall.SockaddrInet4{Addr: addr.Addr().As4(), Port: int(addr.Port())}
	} else {
		domain = syscall.AF_INET6
		sa = &syscall.SockaddrInet6{Addr: addr.Addr().Unmap().As16(), Port: int(addr.Port())}
	}

	sd, err := syscall.Socket(domain, typ|syscall.SOCK_CLOEXEC, int(proto))
	if err != nil {
		return nil, fmt.Errorf("failed to create socket for %s/%s: %w", addr, proto, err)
	}
	defer func() {
		if retErr != nil {
			syscall.Close(sd)
		}
	}()

	if proto == syscall.IPPROTO_TCP {
		if err := syscall.SetsockoptInt(sd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
			return nil, fmt.Errorf("failed to setsockopt(SO_REUSEADDR) for %s/%s: %w", addr, proto, err)
		}
	}

	// We need to listen to make sure that the port is free, and no other process is racing against us to acquire this
	// port. But listening means that connections could be accepted before DNAT rules are inserted, and they'd never
	// reach the container. To avoid this, set a socket filter to drop all connections — TCP SYNs will be
	// re-transmitted anyway. Callers must call DetachSocketFilter.
	//
	// Set the socket filter _before_ binding the socket to make sure that no UDP datagrams will fill the queue.
	if err := setSocketFilter(sd); err != nil {
		return nil, fmt.Errorf("failed to set drop packets filter for %s/%s: %w", addr, proto, err)
	}

	if domain == syscall.AF_INET6 {
		syscall.SetsockoptInt(sd, syscall.IPPROTO_IPV6, syscall.IPV6_V6ONLY, 1)
	}
	if typ == syscall.SOCK_DGRAM {
		// Enable IP_PKTINFO for UDP sockets to get the destination address.
		// The destination address will be used as the source address when
		// sending back replies coming from the container.
		lvl := syscall.IPPROTO_IP
		opt := syscall.IP_PKTINFO
		optName := "IP_PKTINFO"
		if domain == syscall.AF_INET6 {
			lvl = syscall.IPPROTO_IPV6
			opt = syscall.IPV6_RECVPKTINFO
			optName = "IPV6_RECVPKTINFO"
		}
		if err := syscall.SetsockoptInt(sd, lvl, opt, 1); err != nil {
			return nil, fmt.Errorf("failed to setsockopt(%s) for %s/%s: %w", optName, addr, proto, err)
		}
	}
	if err := syscall.Bind(sd, sa); err != nil {
		return nil, fmt.Errorf("failed to bind host port %s/%s: %w", addr, proto, err)
	}

	boundSocket := os.NewFile(uintptr(sd), "listener")
	if boundSocket == nil {
		return nil, fmt.Errorf("failed to convert socket to file for %s/%s", addr, proto)
	}
	return boundSocket, nil
}

// listenSCTP is based on sctp.ListenSCTP.
func listenSCTP(addr netip.AddrPort) (_ *os.File, retErr error) {
	boundSocket, err := bindSCTP(addr)
	if err != nil {
		return nil, err
	}

	somaxconn := -1 // silently capped to "/proc/sys/net/core/somaxconn"
	if err := syscall.Listen(int(boundSocket.Fd()), somaxconn); err != nil {
		return nil, fmt.Errorf("failed to listen on sctp socket: %w", err)
	}

	return boundSocket, nil
}

func bindSCTP(addr netip.AddrPort) (_ *os.File, retErr error) {
	domain := syscall.AF_INET
	if addr.Addr().Unmap().Is6() {
		domain = syscall.AF_INET6
	}

	sd, err := syscall.Socket(domain, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, syscall.IPPROTO_SCTP)
	if err != nil {
		return nil, fmt.Errorf("failed to create socket for %s/sctp: %w", addr, err)
	}
	defer func() {
		if retErr != nil {
			syscall.Close(sd)
		}
	}()

	if domain == syscall.AF_INET6 {
		syscall.SetsockoptInt(sd, syscall.IPPROTO_IPV6, syscall.IPV6_V6ONLY, 1)
	}

	if errno := setSCTPInitMsg(sd, sctp.InitMsg{NumOstreams: sctp.SCTP_MAX_STREAM}); errno != 0 {
		return nil, errno
	}

	if err := sctp.SCTPBind(sd,
		&sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: addr.Addr().Unmap().AsSlice()}}, Port: int(addr.Port())},
		sctp.SCTP_BINDX_ADD_ADDR); err != nil {
		return nil, fmt.Errorf("failed to bind host port %s/sctp: %w", addr, err)
	}

	// We need to listen to make sure that the port is free, and no other process is racing against us to acquire this
	// port. But listening means that connections could be accepted before DNAT rules are inserted, and they'd never
	// reach the container. To avoid this, set a socket filter to drop all connections — SCTP handshake will be
	// re-transmitted anyway. Callers must call DetachSocketFilter.
	if err := setSocketFilter(sd); err != nil {
		return nil, fmt.Errorf("failed to set drop packets filter for %s/sctp: %w", addr, err)
	}

	boundSocket := os.NewFile(uintptr(sd), "listener")
	if boundSocket == nil {
		return nil, fmt.Errorf("failed to convert socket %s/sctp", addr)
	}
	return boundSocket, nil
}

// DetachSocketFilter removes the BPF filter set during port allocation to prevent the kernel from accepting connections
// before DNAT rules are inserted.
func DetachSocketFilter(f *os.File) error {
	return unix.SetsockoptInt(int(f.Fd()), syscall.SOL_SOCKET, syscall.SO_DETACH_FILTER, 0 /* ignored */)
}

// setSocketFilter sets a cBPF program on socket sd to drop all packets. To start receiving packets on this socket,
// callers must call DetachSocketFilter.
func setSocketFilter(sd int) error {
	asm, err := bpf.Assemble([]bpf.Instruction{
		// A cBPF program attached to a socket with SO_ATTACH_FILTER and
		// returning 0 tells the kernel to drop all packets.
		bpf.RetConstant{Val: 0x0},
	})
	if err != nil {
		// (bpf.RetConstant).Assemble() doesn't return an error, so this should
		// be unreachable code.
		return fmt.Errorf("attaching socket filter: %w", err)
	}
	// Make sure the asm slice is not GC'd before setsockopt is called
	defer runtime.KeepAlive(asm)

	if len(asm) == 0 {
		return errors.New("attaching socket filter: empty BPF program")
	}

	f := make([]unix.SockFilter, len(asm))
	for i := range asm {
		f[i] = unix.SockFilter{
			Code: asm[i].Op,
			Jt:   asm[i].Jt,
			Jf:   asm[i].Jf,
			K:    asm[i].K,
		}
	}
	return unix.SetsockoptSockFprog(sd, syscall.SOL_SOCKET, syscall.SO_ATTACH_FILTER, &unix.SockFprog{
		Len:    uint16(len(f)),
		Filter: &f[0],
	})
}
