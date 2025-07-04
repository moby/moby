package portallocator

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"syscall"

	"github.com/containerd/log"
	"github.com/docker/docker/daemon/libnetwork/types"
	"github.com/ishidawataru/sctp"
)

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
// for all the specified addrs, and then try to bind those addresses to allocate
// the port from the OS. It returns the allocated port, and all the sockets
// bound, or an error if the reserved port isn't available. Callers must take
// care of closing the returned sockets.
//
// Due to the semantic of SO_REUSEADDR, the OSAllocator can't fully determine
// if a port is free when binding 0.0.0.0 or ::. If another socket is binding
// the same port, but it's not listening to it yet, the bind will succeed but a
// subsequent listen might fail. For this reason, RequestPortsInRange doesn't
// retry on failure — it's caller's responsibility.
//
// It's safe for concurrent use.
func (pa OSAllocator) RequestPortsInRange(addrs []net.IP, proto types.Protocol, portStart, portEnd int) (_ int, _ []*os.File, retErr error) {
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
		switch proto {
		case types.TCP:
			sock, err = bindTCPOrUDP(addrPort, syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
		case types.UDP:
			sock, err = bindTCPOrUDP(addrPort, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
		case types.SCTP:
			sock, err = bindSCTP(addrPort)
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

	if err := syscall.SetsockoptInt(sd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return nil, fmt.Errorf("failed to setsockopt(SO_REUSEADDR) for %s/%s: %w", addr, proto, err)
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

// bindSCTP is based on sctp.ListenSCTP. The socket is created and bound, but
// does not start listening.
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

	boundSocket := os.NewFile(uintptr(sd), "listener")
	if boundSocket == nil {
		return nil, fmt.Errorf("failed to convert socket %s/sctp", addr)
	}
	return boundSocket, nil
}
