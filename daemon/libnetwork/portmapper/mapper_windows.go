package portmapper

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/moby/moby/v2/daemon/libnetwork/portallocator"
	"github.com/containerd/log"
	"github.com/ishidawataru/sctp"
)

type mapping struct {
	proto             string
	stopUserlandProxy func() error
	host              net.Addr
	container         net.Addr
}

var (
	// ErrUnknownBackendAddressType refers to an unknown container or unsupported address type
	ErrUnknownBackendAddressType = errors.New("unknown container address type not supported")
	// ErrPortMappedForIP refers to a port already mapped to an ip address
	ErrPortMappedForIP = errors.New("port is already mapped to ip")
	// ErrPortNotMapped refers to an unmapped port
	ErrPortNotMapped = errors.New("port is not mapped")
	// ErrSCTPAddrNoIP refers to a SCTP address without IP address.
	ErrSCTPAddrNoIP = errors.New("sctp address does not contain any IP address")
)

// PortMapper manages the network address translation
type PortMapper struct {
	bridgeName string

	// udp:ip:port
	currentMappings map[string]*mapping
	lock            sync.Mutex

	proxyPath string

	allocator *portallocator.PortAllocator
}

// New returns a new instance of PortMapper
func New() *PortMapper {
	return NewWithPortAllocator(portallocator.Get(), "")
}

// NewWithPortAllocator returns a new instance of PortMapper which will use the specified PortAllocator
func NewWithPortAllocator(allocator *portallocator.PortAllocator, proxyPath string) *PortMapper {
	return &PortMapper{
		currentMappings: make(map[string]*mapping),
		allocator:       allocator,
		proxyPath:       proxyPath,
	}
}

// MapRange maps the specified container transport address to the host's network address and transport port range
func (pm *PortMapper) MapRange(container net.Addr, hostIP net.IP, hostPortStart, hostPortEnd int) (host net.Addr, retErr error) {
	pm.lock.Lock()
	defer pm.lock.Unlock()

	var (
		m                 *mapping
		proto             string
		allocatedHostPort int
	)

	switch container.(type) {
	case *net.TCPAddr:
		proto = "tcp"

		var err error
		allocatedHostPort, err = pm.allocator.RequestPortInRange(hostIP, proto, hostPortStart, hostPortEnd)
		if err != nil {
			return nil, err
		}
		defer func() {
			if retErr != nil {
				pm.allocator.ReleasePort(hostIP, proto, allocatedHostPort)
			}
		}()

		m = &mapping{
			proto:     proto,
			host:      &net.TCPAddr{IP: hostIP, Port: allocatedHostPort},
			container: container,
		}
	case *net.UDPAddr:
		proto = "udp"

		var err error
		allocatedHostPort, err = pm.allocator.RequestPortInRange(hostIP, proto, hostPortStart, hostPortEnd)
		if err != nil {
			return nil, err
		}
		defer func() {
			if retErr != nil {
				pm.allocator.ReleasePort(hostIP, proto, allocatedHostPort)
			}
		}()

		m = &mapping{
			proto:     proto,
			host:      &net.UDPAddr{IP: hostIP, Port: allocatedHostPort},
			container: container,
		}
	case *sctp.SCTPAddr:
		proto = "sctp"

		var err error
		allocatedHostPort, err = pm.allocator.RequestPortInRange(hostIP, proto, hostPortStart, hostPortEnd)
		if err != nil {
			return nil, err
		}
		defer func() {
			if retErr != nil {
				pm.allocator.ReleasePort(hostIP, proto, allocatedHostPort)
			}
		}()

		m = &mapping{
			proto:     proto,
			host:      &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: hostIP}}, Port: allocatedHostPort},
			container: container,
		}
	default:
		return nil, ErrUnknownBackendAddressType
	}

	key := getKey(m.host)
	if _, exists := pm.currentMappings[key]; exists {
		return nil, ErrPortMappedForIP
	}

	var err error
	m.stopUserlandProxy, err = newDummyProxy(m.proto, hostIP, allocatedHostPort)
	if err != nil {
		// FIXME(thaJeztah): both stopping the proxy and deleting iptables rules can produce an error, and both are not currently handled.
		m.stopUserlandProxy()
		return nil, err
	}

	pm.currentMappings[key] = m
	return m.host, nil
}

// Unmap removes stored mapping for the specified host transport address
func (pm *PortMapper) Unmap(host net.Addr) error {
	pm.lock.Lock()
	defer pm.lock.Unlock()

	key := getKey(host)
	data, exists := pm.currentMappings[key]
	if !exists {
		return ErrPortNotMapped
	}

	if data.stopUserlandProxy != nil {
		data.stopUserlandProxy()
	}

	delete(pm.currentMappings, key)

	switch a := host.(type) {
	case *net.TCPAddr:
		pm.allocator.ReleasePort(a.IP, "tcp", a.Port)
	case *net.UDPAddr:
		pm.allocator.ReleasePort(a.IP, "udp", a.Port)
	case *sctp.SCTPAddr:
		if len(a.IPAddrs) == 0 {
			return ErrSCTPAddrNoIP
		}
		pm.allocator.ReleasePort(a.IPAddrs[0].IP, "sctp", a.Port)
	default:
		return ErrUnknownBackendAddressType
	}

	return nil
}

func getKey(a net.Addr) string {
	switch t := a.(type) {
	case *net.TCPAddr:
		return fmt.Sprintf("%s:%d/%s", t.IP.String(), t.Port, "tcp")
	case *net.UDPAddr:
		return fmt.Sprintf("%s:%d/%s", t.IP.String(), t.Port, "udp")
	case *sctp.SCTPAddr:
		if len(t.IPAddrs) == 0 {
			log.G(context.TODO()).Error(ErrSCTPAddrNoIP)
			return ""
		}
		return fmt.Sprintf("%s:%d/%s", t.IPAddrs[0].IP.String(), t.Port, "sctp")
	}
	return ""
}

func getIPAndPort(a net.Addr) (net.IP, int) {
	switch t := a.(type) {
	case *net.TCPAddr:
		return t.IP, t.Port
	case *net.UDPAddr:
		return t.IP, t.Port
	case *sctp.SCTPAddr:
		if len(t.IPAddrs) == 0 {
			log.G(context.TODO()).Error(ErrSCTPAddrNoIP)
			return nil, 0
		}
		return t.IPAddrs[0].IP, t.Port
	}
	return nil, 0
}
