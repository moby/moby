package portmapper

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/networkdriver/portallocator"
	"github.com/docker/docker/pkg/iptables"
)

type mapping struct {
	proto         string
	userlandProxy UserlandProxy
	host          net.Addr
	container     net.Addr
}

var NewProxy = NewProxyCommand

var (
	ErrUnknownBackendAddressType = errors.New("unknown container address type not supported")
	ErrPortMappedForIP           = errors.New("port is already mapped to ip")
	ErrPortNotMapped             = errors.New("port is not mapped")
)

type PortMapper struct {
	chain *iptables.Chain

	// udp:ip:port
	currentMappings map[string]*mapping
	lock            sync.Mutex

	Allocator *portallocator.PortAllocator
}

func New() *PortMapper {
	return NewWithPortAllocator(portallocator.New())
}

func NewWithPortAllocator(allocator *portallocator.PortAllocator) *PortMapper {
	return &PortMapper{
		currentMappings: make(map[string]*mapping),
		Allocator:       allocator,
	}
}

func (pm *PortMapper) SetIptablesChain(c *iptables.Chain) {
	pm.chain = c
}

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
			m.userlandProxy = NewProxy(proto, hostIP, allocatedHostPort, container.(*net.TCPAddr).IP, container.(*net.TCPAddr).Port)
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
			m.userlandProxy = NewProxy(proto, hostIP, allocatedHostPort, container.(*net.UDPAddr).IP, container.(*net.UDPAddr).Port)
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
		if m.userlandProxy != nil {
			m.userlandProxy.Stop()
		}
		pm.forward(iptables.Delete, m.proto, hostIP, allocatedHostPort, containerIP.String(), containerPort)
		if err := pm.Allocator.ReleasePort(hostIP, m.proto, allocatedHostPort); err != nil {
			return err
		}

		return nil
	}

	if m.userlandProxy != nil {
		if err := m.userlandProxy.Start(); err != nil {
			if err := cleanup(); err != nil {
				return nil, fmt.Errorf("Error during port allocation cleanup: %v", err)
			}
			return nil, err
		}
	}

	pm.currentMappings[key] = m
	return m.host, nil
}

// re-apply all port mappings
func (pm *PortMapper) ReMapAll() {
	logrus.Debugln("Re-applying all port mappings.")
	for _, data := range pm.currentMappings {
		containerIP, containerPort := getIPAndPort(data.container)
		hostIP, hostPort := getIPAndPort(data.host)
		if err := pm.forward(iptables.Append, data.proto, hostIP, hostPort, containerIP.String(), containerPort); err != nil {
			logrus.Errorf("Error on iptables add: %s", err)
		}
	}
}

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
