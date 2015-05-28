package portmapper

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/iptables"
	"github.com/docker/libnetwork/portallocator"
)

type mapping struct {
	proto         string
	userlandProxy userlandProxy
	host          net.Addr
	container     net.Addr
}

var newProxy = newProxyCommand

var (
	// ErrUnknownBackendAddressType refers to an unknown container or unsupported address type
	ErrUnknownBackendAddressType = errors.New("unknown container address type not supported")
	// ErrPortMappedForIP refers to a port already mapped to an ip address
	ErrPortMappedForIP = errors.New("port is already mapped to ip")
	// ErrPortNotMapped refers to an unmapped port
	ErrPortNotMapped = errors.New("port is not mapped")
)

// PortMapper manages the network address translation
type PortMapper struct {
	chain *iptables.Chain

	// udp:ip:port
	currentMappings map[string]*mapping
	lock            sync.Mutex

	Allocator *portallocator.PortAllocator
}

// New returns a new instance of PortMapper
func New() *PortMapper {
	return NewWithPortAllocator(portallocator.Get())
}

// NewWithPortAllocator returns a new instance of PortMapper which will use the specified PortAllocator
func NewWithPortAllocator(allocator *portallocator.PortAllocator) *PortMapper {
	return &PortMapper{
		currentMappings: make(map[string]*mapping),
		Allocator:       allocator,
	}
}

// SetIptablesChain sets the specified chain into portmapper
func (pm *PortMapper) SetIptablesChain(c *iptables.Chain) {
	pm.chain = c
}

// Map maps the specified container transport address to the host's network address and transport port
func (pm *PortMapper) Map(container net.Addr, hostIP net.IP, hostPort int, useProxy bool) (host net.Addr, err error) {
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
		if allocatedHostPort, err = pm.Allocator.RequestPort(hostIP, proto, hostPort); err != nil {
			return nil, err
		}

		m = &mapping{
			proto:     proto,
			host:      &net.TCPAddr{IP: hostIP, Port: allocatedHostPort},
			container: container,
		}

		if useProxy {
			m.userlandProxy = newProxy(proto, hostIP, allocatedHostPort, container.(*net.TCPAddr).IP, container.(*net.TCPAddr).Port)
		} else {
			m.userlandProxy = newDummyProxy(proto, hostIP, allocatedHostPort)
		}
	case *net.UDPAddr:
		proto = "udp"
		if allocatedHostPort, err = pm.Allocator.RequestPort(hostIP, proto, hostPort); err != nil {
			return nil, err
		}

		m = &mapping{
			proto:     proto,
			host:      &net.UDPAddr{IP: hostIP, Port: allocatedHostPort},
			container: container,
		}

		if useProxy {
			m.userlandProxy = newProxy(proto, hostIP, allocatedHostPort, container.(*net.UDPAddr).IP, container.(*net.UDPAddr).Port)
		} else {
			m.userlandProxy = newDummyProxy(proto, hostIP, allocatedHostPort)
		}
	default:
		return nil, ErrUnknownBackendAddressType
	}

	// release the allocated port on any further error during return.
	defer func() {
		if err != nil {
			pm.Allocator.ReleasePort(hostIP, proto, allocatedHostPort)
		}
	}()

	key := getKey(m.host)
	if _, exists := pm.currentMappings[key]; exists {
		return nil, ErrPortMappedForIP
	}

	containerIP, containerPort := getIPAndPort(m.container)
	if err := pm.forward(iptables.Append, m.proto, hostIP, allocatedHostPort, containerIP.String(), containerPort); err != nil {
		return nil, err
	}

	cleanup := func() error {
		// need to undo the iptables rules before we return
		m.userlandProxy.Stop()
		pm.forward(iptables.Delete, m.proto, hostIP, allocatedHostPort, containerIP.String(), containerPort)
		if err := pm.Allocator.ReleasePort(hostIP, m.proto, allocatedHostPort); err != nil {
			return err
		}

		return nil
	}

	if err := m.userlandProxy.Start(); err != nil {
		if err := cleanup(); err != nil {
			return nil, fmt.Errorf("Error during port allocation cleanup: %v", err)
		}
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

	if data.userlandProxy != nil {
		data.userlandProxy.Stop()
	}

	delete(pm.currentMappings, key)

	containerIP, containerPort := getIPAndPort(data.container)
	hostIP, hostPort := getIPAndPort(data.host)
	if err := pm.forward(iptables.Delete, data.proto, hostIP, hostPort, containerIP.String(), containerPort); err != nil {
		logrus.Errorf("Error on iptables delete: %s", err)
	}

	switch a := host.(type) {
	case *net.TCPAddr:
		return pm.Allocator.ReleasePort(a.IP, "tcp", a.Port)
	case *net.UDPAddr:
		return pm.Allocator.ReleasePort(a.IP, "udp", a.Port)
	}
	return nil
}

func getKey(a net.Addr) string {
	switch t := a.(type) {
	case *net.TCPAddr:
		return fmt.Sprintf("%s:%d/%s", t.IP.String(), t.Port, "tcp")
	case *net.UDPAddr:
		return fmt.Sprintf("%s:%d/%s", t.IP.String(), t.Port, "udp")
	}
	return ""
}

func getIPAndPort(a net.Addr) (net.IP, int) {
	switch t := a.(type) {
	case *net.TCPAddr:
		return t.IP, t.Port
	case *net.UDPAddr:
		return t.IP, t.Port
	}
	return nil, 0
}

func (pm *PortMapper) forward(action iptables.Action, proto string, sourceIP net.IP, sourcePort int, containerIP string, containerPort int) error {
	if pm.chain == nil {
		return nil
	}
	return pm.chain.Forward(action, sourceIP, sourcePort, proto, containerIP, containerPort)
}
