package portmapper

import (
	"errors"
	"fmt"
	"net"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/networkdriver/portallocator"
	"github.com/docker/docker/pkg/iptables"
)

type mapping struct {
	proto         string
	userlandProxy UserlandProxy
	host          net.Addr
	container     net.Addr
}

var (
	chain *iptables.Chain
	lock  sync.Mutex

	// udp:ip:port
	currentMappings = make(map[string]*mapping)

	NewProxy = NewProxyCommand
)

var (
	ErrUnknownBackendAddressType = errors.New("unknown container address type not supported")
	ErrPortMappedForIP           = errors.New("port is already mapped to ip")
	ErrPortNotMapped             = errors.New("port is not mapped")
)

func SetIptablesChain(c *iptables.Chain) {
	chain = c
}

func Map(ipaddr net.IP, proto string, containerPort int, hostIP net.IP, hostPort int) (hosts []net.Addr, err error) {
	lock.Lock()
	defer lock.Unlock()

	var (
		m                 *mapping
		allocatedHostPort int
		proxy             UserlandProxy
		container         net.Addr
	)

	// host ip, proto, and host port
	switch proto {
	case "tcp":
		container = &net.TCPAddr{IP: ipaddr, Port: containerPort}
		if allocatedHostPort, err = portallocator.RequestPort(hostIP, proto, hostPort); err != nil {
			return nil, err
		}

		m = &mapping{
			proto:     proto,
			host:      &net.TCPAddr{IP: hostIP, Port: allocatedHostPort},
			container: container,
		}
		proxy = NewProxy(proto, hostIP, allocatedHostPort, container.(*net.TCPAddr).IP, container.(*net.TCPAddr).Port)
		host, err := updateIPTables(m, proxy, hostIP, allocatedHostPort)
		if err != nil {
			return nil, err
		}
		return []net.Addr{host}, nil
	case "udp":
		container = &net.UDPAddr{IP: ipaddr, Port: containerPort}
		if allocatedHostPort, err = portallocator.RequestPort(hostIP, proto, hostPort); err != nil {
			return nil, err
		}

		m = &mapping{
			proto:     proto,
			host:      &net.UDPAddr{IP: hostIP, Port: allocatedHostPort},
			container: container,
		}
		proxy = NewProxy(proto, hostIP, allocatedHostPort, container.(*net.UDPAddr).IP, container.(*net.UDPAddr).Port)
		host, err := updateIPTables(m, proxy, hostIP, allocatedHostPort)
		if err != nil {
			return nil, err
		}
		return []net.Addr{host}, nil

	case "udptcp":
	case "tcpudp":
		proto = "tcp"
		container = &net.TCPAddr{IP: ipaddr, Port: containerPort}
		if allocatedHostPort, err = portallocator.RequestTCPUDPPort(hostIP, proto, hostPort); err != nil {
			return nil, err
		}

		m = &mapping{
			proto:     proto,
			host:      &net.TCPAddr{IP: hostIP, Port: allocatedHostPort},
			container: container,
		}

		proxy = NewProxy(proto, hostIP, allocatedHostPort, container.(*net.TCPAddr).IP, container.(*net.TCPAddr).Port)
		host, err := updateIPTables(m, proxy, hostIP, allocatedHostPort)
		if err != nil {
			return nil, err
		}

		proto = "udp"
		container = &net.UDPAddr{IP: ipaddr, Port: containerPort}
		m = &mapping{
			proto:     proto,
			host:      &net.UDPAddr{IP: hostIP, Port: allocatedHostPort},
			container: container,
		}

		proxy = NewProxy(proto, hostIP, allocatedHostPort, container.(*net.UDPAddr).IP, container.(*net.UDPAddr).Port)
		host2, err := updateIPTables(m, proxy, hostIP, allocatedHostPort)
		if err != nil {
			return nil, err
		}
		return []net.Addr{host, host2}, nil

	default:
		return nil, ErrUnknownBackendAddressType
	}
	return nil, errors.New("Unknown Error")
}

func updateIPTables(m *mapping, proxy UserlandProxy, hostIP net.IP, allocatedHostPort int) (host net.Addr, err error) {

	// release the allocated port on any further error during return.
	defer func() {
		if err != nil {
			portallocator.ReleasePort(hostIP, m.proto, allocatedHostPort)
		}
	}()

	key := getKey(m.host)
	if _, exists := currentMappings[key]; exists {
		return nil, ErrPortMappedForIP
	}

	containerIP, containerPort := getIPAndPort(m.container)
	if err := forward(iptables.Add, m.proto, hostIP, allocatedHostPort, containerIP.String(), containerPort); err != nil {
		return nil, err
	}

	cleanup := func() error {
		// need to undo the iptables rules before we return
		proxy.Stop()
		forward(iptables.Delete, m.proto, hostIP, allocatedHostPort, containerIP.String(), containerPort)
		if err := portallocator.ReleasePort(hostIP, m.proto, allocatedHostPort); err != nil {
			return err
		}

		return nil
	}

	if err := proxy.Start(); err != nil {
		if err := cleanup(); err != nil {
			return nil, fmt.Errorf("Error during port allocation cleanup: %v", err)
		}
		return nil, err
	}
	m.userlandProxy = proxy
	currentMappings[key] = m
	return m.host, nil
}

func Unmap(host net.Addr) error {
	lock.Lock()
	defer lock.Unlock()

	key := getKey(host)
	data, exists := currentMappings[key]
	if !exists {
		return ErrPortNotMapped
	}

	data.userlandProxy.Stop()

	delete(currentMappings, key)

	containerIP, containerPort := getIPAndPort(data.container)
	hostIP, hostPort := getIPAndPort(data.host)
	if err := forward(iptables.Delete, data.proto, hostIP, hostPort, containerIP.String(), containerPort); err != nil {
		log.Errorf("Error on iptables delete: %s", err)
	}

	switch a := host.(type) {
	case *net.TCPAddr:
		return portallocator.ReleasePort(a.IP, "tcp", a.Port)
	case *net.UDPAddr:
		return portallocator.ReleasePort(a.IP, "udp", a.Port)
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

func forward(action iptables.Action, proto string, sourceIP net.IP, sourcePort int, containerIP string, containerPort int) error {
	if chain == nil {
		return nil
	}
	return chain.Forward(action, sourceIP, sourcePort, proto, containerIP, containerPort)
}
