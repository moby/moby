package portmapper

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"

	"github.com/containerd/log"
	"github.com/ishidawataru/sctp"
	"github.com/moby/moby/v2/daemon/libnetwork/portallocator"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

var (
	// ErrPortMappedForIP refers to a port already mapped to an ip address
	ErrPortMappedForIP = errors.New("port is already mapped to ip")
	// ErrPortNotMapped refers to an unmapped port
	ErrPortNotMapped = errors.New("port is not mapped")
)

// PortMapper manages the network address translation
type PortMapper struct {
	// osListeners stores listening sockets used by active port mappings
	// to reserve ports from the OS. Outer map is keyed by protocol, and inner
	// map is keyed by host address and port.
	osListeners map[types.Protocol]map[netip.AddrPort]io.Closer
	lock        sync.Mutex

	allocator *portallocator.PortAllocator
}

// New returns a new instance of PortMapper
func New() *PortMapper {
	return &PortMapper{
		osListeners: make(map[types.Protocol]map[netip.AddrPort]io.Closer),
		allocator:   portallocator.Get(),
	}
}

// MapRange maps the specified container transport address to the host's network address and transport port range
func (pm *PortMapper) MapRange(hostIP net.IP, proto types.Protocol, hostPortStart, hostPortEnd int) (_ int, retErr error) {
	pm.lock.Lock()
	defer pm.lock.Unlock()

	allocatedHostPort, err := pm.allocator.RequestPortInRange(hostIP, proto.String(), hostPortStart, hostPortEnd)
	if err != nil {
		return 0, err
	}
	defer func() {
		if retErr != nil {
			pm.allocator.ReleasePort(hostIP, proto.String(), allocatedHostPort)
		}
	}()

	if pm.osListeners[proto] == nil {
		pm.osListeners[proto] = make(map[netip.AddrPort]io.Closer)
	}

	addr, ok := netip.AddrFromSlice(hostIP)
	if !ok {
		return 0, fmt.Errorf("invalid HostIP: %s", hostIP)
	}

	hAddrPort := netip.AddrPortFrom(addr, uint16(allocatedHostPort))
	if _, exists := pm.osListeners[proto][hAddrPort]; exists {
		return 0, ErrPortMappedForIP
	}

	var osListener io.Closer
	osListener, err = allocateHostPort(proto.String(), hostIP, allocatedHostPort)
	if err != nil {
		if osListener != nil {
			if err := osListener.Close(); err != nil {
				// Prior to v29.0, this error was never checked. So, instead of
				// returning an error, log it and proceed.
				log.G(context.TODO()).Infof("failed to stop dummy proxy for %s/%s: %v", hostIP, proto, err)
			}
		}
		return 0, err
	}

	pm.osListeners[proto][hAddrPort] = osListener
	return allocatedHostPort, nil
}

// allocateHostPort allocates a host port by binding to the specified host IP and port.
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

// Unmap removes stored mapping for the specified host transport address
func (pm *PortMapper) Unmap(hostIP net.IP, proto types.Protocol, hostPort int) error {
	pm.lock.Lock()
	defer pm.lock.Unlock()

	addr, ok := netip.AddrFromSlice(hostIP)
	if !ok {
		return fmt.Errorf("invalid HostIP: %s", hostIP)
	}

	if pm.osListeners[proto] == nil {
		return ErrPortNotMapped
	}

	hAddrPort := netip.AddrPortFrom(addr, uint16(hostPort))
	osListener, exists := pm.osListeners[proto][hAddrPort]
	if !exists {
		return ErrPortNotMapped
	}

	if osListener != nil {
		if err := osListener.Close(); err != nil {
			// Prior to v29.0, this error was never checked. So, instead of
			// returning an error, log it and proceed.
			log.G(context.TODO()).Infof("failed to stop dummy proxy for %s/%s: %v", hostIP, proto, err)
		}
	}

	delete(pm.osListeners[proto], hAddrPort)

	pm.allocator.ReleasePort(hostIP, proto.String(), int(hostPort))
	return nil
}
