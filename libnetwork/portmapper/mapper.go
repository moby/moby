//go:build windows

package portmapper

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/portallocator"
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

	containerIP, containerPort := getIPAndPort(m.container)
	if err := pm.AppendForwardingTableEntry(m.proto, hostIP, allocatedHostPort, containerIP.String(), containerPort); err != nil {
		return nil, err
	}

	var err error
	m.stopUserlandProxy, err = newDummyProxy(m.proto, hostIP, allocatedHostPort)
	if err != nil {
		// FIXME(thaJeztah): both stopping the proxy and deleting iptables rules can produce an error, and both are not currently handled.
		m.stopUserlandProxy()
		// need to undo the iptables rules before we return
		pm.DeleteForwardingTableEntry(m.proto, hostIP, allocatedHostPort, containerIP.String(), containerPort)
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

	containerIP, containerPort := getIPAndPort(data.container)
	hostIP, hostPort := getIPAndPort(data.host)
	if err := pm.DeleteForwardingTableEntry(data.proto, hostIP, hostPort, containerIP.String(), containerPort); err != nil {
		log.G(context.TODO()).Errorf("Error on iptables delete: %s", err)
	}

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
