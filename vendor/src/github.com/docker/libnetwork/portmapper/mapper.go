package portmapper

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/iptables"
	"github.com/docker/libnetwork/ipvs"
	"github.com/docker/libnetwork/portallocator"
	"github.com/vishvananda/netlink"
)

type mapping struct {
	proto         string
	userlandProxy userlandProxy
	host          net.Addr
	container     net.Addr
}

var (
	newProxy   = newProxyCommand
	loopbackIP = net.ParseIP("127.0.0.1")
)

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
	chain      *iptables.ChainInfo
	bridgeName string

	// udp:ip:port
	currentMappings map[string]*mapping
	lock            sync.Mutex

	Allocator *portallocator.PortAllocator
	ipvs      *ipvs.Handle
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
func (pm *PortMapper) SetIptablesChain(c *iptables.ChainInfo, bridgeName string) {
	pm.chain = c
	pm.bridgeName = bridgeName
}

// SetupIPVS establishes the IPVS handle which is used for setting up NATs for external traffic
func (pm *PortMapper) SetupIPVS() error {
	ipvs, err := ipvs.New("")
	if err != nil {
		return err
	}
	pm.ipvs = ipvs
	return nil
}

// Map maps the specified container transport address to the host's network address and transport port
func (pm *PortMapper) Map(container net.Addr, hostIP net.IP, hostPort int, useProxy bool) (host net.Addr, err error) {
	return pm.MapRange(container, hostIP, hostPort, hostPort, useProxy)
}

// MapRange maps the specified container transport address to the host's network address and transport port range
func (pm *PortMapper) MapRange(container net.Addr, hostIP net.IP, hostPortStart, hostPortEnd int, useProxy bool) (host net.Addr, err error) {
	pm.lock.Lock()
	defer pm.lock.Unlock()

	var (
		m                 *mapping
		proto             string
		allocatedHostPort int
	)

	switch addr := container.(type) {
	case *net.TCPAddr:
		proto = "tcp"
		if allocatedHostPort, err = pm.Allocator.RequestPortInRange(hostIP, proto, hostPortStart, hostPortEnd); err != nil {
			return nil, err
		}

		m = &mapping{
			proto:     proto,
			host:      &net.TCPAddr{IP: hostIP, Port: allocatedHostPort},
			container: container,
		}

		if useProxy {
			m.userlandProxy, err = newProxy(proto, hostIP, allocatedHostPort, addr.IP, addr.Port)
			if err != nil {
				return nil, err
			}
		} else {
			m.userlandProxy = newDummyProxy(proto, hostIP, allocatedHostPort)
		}
	case *net.UDPAddr:
		proto = "udp"
		if allocatedHostPort, err = pm.Allocator.RequestPortInRange(hostIP, proto, hostPortStart, hostPortEnd); err != nil {
			return nil, err
		}

		m = &mapping{
			proto:     proto,
			host:      &net.UDPAddr{IP: hostIP, Port: allocatedHostPort},
			container: container,
		}

		if useProxy {
			m.userlandProxy, err = newProxy(proto, hostIP, allocatedHostPort, addr.IP, addr.Port)
			if err != nil {
				return nil, err
			}
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
	fwMark := getFWMark(m.proto, allocatedHostPort)

	cleanup := func() error {
		// need to undo the iptables rules before we return
		m.userlandProxy.Stop()
		if !hostIP.IsLoopback() {
			pm.deleteService(fwMark)
		}
		pm.forward(iptables.Delete, m.proto, hostIP, allocatedHostPort, containerIP, containerPort, fwMark)
		if err := pm.Allocator.ReleasePort(hostIP, m.proto, allocatedHostPort); err != nil {
			return err
		}
		return nil
	}

	// loopback can't use ipvs
	if !hostIP.IsLoopback() {
		if err := pm.createService(allocatedHostPort, containerIP, containerPort, fwMark); err != nil {
			if err := cleanup(); err != nil {
				logrus.Warnf("Error while cleaning up port forwards: %v", err)
			}
		}
	}

	// Need to setup special routing for localhost which can't go through ipvs
	if err := pm.forward(iptables.Append, m.proto, hostIP, allocatedHostPort, containerIP, containerPort, fwMark); err != nil {
		if err := cleanup(); err != nil {
			logrus.Warnf("Error while cleaning up port forwards: %v", err)
		}
		return nil, err
	}

	if err := m.userlandProxy.Start(); err != nil {
		if err := cleanup(); err != nil {
			logrus.Error("Error during port allocation cleanup: %v", err)
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
	fwMark := getFWMark(data.proto, hostPort)

	if err := pm.forward(iptables.Delete, data.proto, hostIP, hostPort, containerIP, containerPort, fwMark); err != nil {
		logrus.Errorf("Error cleaning up port map: %s", err)
	}

	if !hostIP.IsLoopback() {
		if err := pm.deleteService(fwMark); err != nil {
			logrus.Errorf("Error removing port map: %s", err)
		}
	}

	switch a := host.(type) {
	case *net.TCPAddr:
		return pm.Allocator.ReleasePort(a.IP, "tcp", a.Port)
	case *net.UDPAddr:
		return pm.Allocator.ReleasePort(a.IP, "udp", a.Port)
	}
	return nil
}

//ReMapAll will re-apply all port mappings
// this is only used by firewalld
func (pm *PortMapper) ReMapAll() {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	logrus.Debugln("Re-applying all port mappings.")
	for _, data := range pm.currentMappings {
		containerIP, containerPort := getIPAndPort(data.container)
		hostIP, hostPort := getIPAndPort(data.host)
		fwMark := getFWMark(data.proto, containerPort)

		if err := pm.forward(iptables.Append, data.proto, hostIP, hostPort, containerIP, containerPort, fwMark); err != nil {
			logrus.Errorf("Error on iptables add: %s", err)
		}
	}
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

func (pm *PortMapper) forward(action iptables.Action, proto string, hostIP net.IP, hostPort int, containerIP net.IP, containerPort int, mark int) error {
	if pm.chain == nil {
		return nil
	}
	isLoopback := hostIP.IsLoopback()
	isUnspec := hostIP.IsUnspecified()

	if !isLoopback {
		// Can't use ipvs for loopback, so no need to mark
		if err := pm.chain.Mark(action, proto, hostIP, hostPort, mark); err != nil {
			return err
		}
	}

	// with ipvs, we only need to forward on localhost and the actual hairpin
	if pm.chain.HairpinMode {
		if isUnspec || isLoopback {
			if err := pm.chain.Forward(action, loopbackIP, hostPort, proto, containerIP, containerPort, pm.bridgeName); err != nil {
				return err
			}
		}

		if !isLoopback {
			// allow the container to talk to itself over the nat'd address
			// ipvs does not support hairpin
			if err := pm.chain.Hairpin(action, proto, hostIP, hostPort, containerIP, containerPort); err != nil {
				return err
			}
		}
	}
	return pm.chain.Masq(action, proto, containerIP, containerPort)
}

func (pm *PortMapper) createService(hostPort int, containerIP net.IP, containerPort, mark int) error {
	service := &ipvs.Service{
		FWMark:        uint32(mark),
		AddressFamily: netlink.FAMILY_V4,
		SchedName:     ipvs.LeastConnection,
	}
	if err := pm.ipvs.NewService(service); err != nil {
		return err
	}
	dest := &ipvs.Destination{
		AddressFamily:   netlink.FAMILY_V4,
		Address:         containerIP,
		Port:            uint16(containerPort),
		ConnectionFlags: ipvs.ConnectionFlagMasq,
		Weight:          1,
	}
	return pm.ipvs.NewDestination(service, dest)
}

func (pm *PortMapper) deleteService(mark int) error {
	return pm.ipvs.DelService(&ipvs.Service{
		FWMark:        uint32(mark),
		AddressFamily: netlink.FAMILY_V4,
		SchedName:     ipvs.LeastConnection,
	})
}

func getFWMark(proto string, port int) int {
	var prefix int
	switch proto {
	case "udp":
		prefix = 5500000
	case "tcp":
		prefix = 5600000
	}
	return prefix + port
}
