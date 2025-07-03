package portallocator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/types"
	"github.com/ishidawataru/sctp"
)

var (
	// ErrPortMappedForIP refers to a port already mapped to an ip address
	ErrPortMappedForIP = errors.New("port is already mapped to ip")
	// ErrPortNotMapped refers to an unmapped port
	ErrPortNotMapped = errors.New("port is not mapped")
)

const (
	// defaultPortRangeStart indicates the first port in port range
	defaultPortRangeStart = 60000
	// defaultPortRangeEnd indicates the last port in port range
	defaultPortRangeEnd = 65000
)

func getDynamicPortRange() (start int, end int, _ error) {
	return defaultPortRangeStart, defaultPortRangeEnd, nil
}

// OSAllocator manages the network address translation
type OSAllocator struct {
	// allocatedPorts stores listening sockets used by active port mappings
	// to reserve ports from the OS. Outer map is keyed by protocol, and inner
	// map is keyed by host address and port.
	allocatedPorts map[types.Protocol]map[netip.AddrPort]io.Closer
	lock           sync.Mutex

	allocator *PortAllocator
}

// New returns a new instance of OSAllocator
func New() *OSAllocator {
	return &OSAllocator{
		allocatedPorts: make(map[types.Protocol]map[netip.AddrPort]io.Closer),
		allocator:      Get(),
	}
}

// AllocateHostPort allocates a port from the OS (by creating a listening socket).
func (pa *OSAllocator) AllocateHostPort(hostIP net.IP, proto types.Protocol, hostPortStart, hostPortEnd int) (_ int, retErr error) {
	pa.lock.Lock()
	defer pa.lock.Unlock()

	allocatedHostPort, err := pa.allocator.RequestPortInRange(hostIP, proto.String(), hostPortStart, hostPortEnd)
	if err != nil {
		return 0, err
	}
	defer func() {
		if retErr != nil {
			pa.allocator.ReleasePort(hostIP, proto.String(), allocatedHostPort)
		}
	}()

	if pa.allocatedPorts[proto] == nil {
		pa.allocatedPorts[proto] = make(map[netip.AddrPort]io.Closer)
	}

	addr, ok := netip.AddrFromSlice(hostIP)
	if !ok {
		return 0, fmt.Errorf("invalid HostIP: %s", hostIP)
	}

	hAddrPort := netip.AddrPortFrom(addr, uint16(allocatedHostPort))
	if _, exists := pa.allocatedPorts[proto][hAddrPort]; exists {
		return 0, ErrPortMappedForIP
	}

	var allocatedPort io.Closer
	allocatedPort, err = allocateHostPort(proto.String(), hostIP, allocatedHostPort)
	if err != nil {
		if allocatedPort != nil {
			if err := allocatedPort.Close(); err != nil {
				// Prior to v29.0, this error was never checked. So, instead of
				// returning an error, log it and proceed.
				log.G(context.TODO()).Infof("failed to stop dummy proxy for %s/%s: %v", hostIP, proto, err)
			}
		}
		return 0, err
	}

	pa.allocatedPorts[proto][hAddrPort] = allocatedPort
	return allocatedHostPort, nil
}

func allocateHostPort(proto string, hostIP net.IP, hostPort int) (io.Closer, error) {
	// detect version of hostIP to bind only to correct version
	protoVer := proto + "4"
	if hostIP.To4() == nil {
		protoVer = proto + "6"
	}

	switch proto {
	case "tcp":
		l, err := net.ListenTCP(protoVer, &net.TCPAddr{IP: hostIP, Port: hostPort})
		if err != nil {
			return nil, err
		}
		return l, nil
	case "udp":
		l, err := net.ListenUDP(protoVer, &net.UDPAddr{IP: hostIP, Port: hostPort})
		if err != nil {
			return nil, err
		}
		return l, nil
	case "sctp":
		l, err := sctp.ListenSCTP(protoVer, &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: hostIP}}, Port: hostPort})
		if err != nil {
			return nil, err
		}
		return l, nil
	default:
		return nil, fmt.Errorf("protocol %s not supported", proto)
	}
}

// Deallocate removes stored mapping for the specified host transport address
func (pa *OSAllocator) Deallocate(hostIP net.IP, proto types.Protocol, hostPort int) error {
	pa.lock.Lock()
	defer pa.lock.Unlock()

	addr, ok := netip.AddrFromSlice(hostIP)
	if !ok {
		return fmt.Errorf("invalid HostIP: %s", hostIP)
	}

	if pa.allocatedPorts[proto] == nil {
		return ErrPortNotMapped
	}

	hAddrPort := netip.AddrPortFrom(addr, uint16(hostPort))
	allocatedPort, exists := pa.allocatedPorts[proto][hAddrPort]
	if !exists {
		return ErrPortNotMapped
	}

	if allocatedPort != nil {
		if err := allocatedPort.Close(); err != nil {
			// Prior to v29.0, this error was never checked. So, instead of
			// returning an error, log it and proceed.
			log.G(context.TODO()).Infof("failed to stop dummy proxy for %s/%s: %v", hostIP, proto, err)
		}
	}

	delete(pa.allocatedPorts[proto], hAddrPort)

	pa.allocator.ReleasePort(hostIP, proto.String(), int(hostPort))
	return nil
}
