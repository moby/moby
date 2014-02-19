package portmapper

import (
	"errors"
	"fmt"
	"github.com/dotcloud/docker/pkg/iptables"
	"github.com/dotcloud/docker/pkg/proxy"
	"net"
	"sync"
)

type mapping struct {
	proto         string
	userlandProxy proxy.Proxy
	host          net.Addr
	container     net.Addr
}

var (
	chain *iptables.Chain
	lock  sync.Mutex

	// udp:ip:port
	currentMappings = make(map[string]*mapping)
	newProxy        = proxy.NewProxy
)

var (
	ErrUnknownBackendAddressType = errors.New("unknown container address type not supported")
	ErrPortMappedForIP           = errors.New("port is already mapped to ip")
	ErrPortNotMapped             = errors.New("port is not mapped")
)

func SetIptablesChain(c *iptables.Chain) {
	chain = c
}

func Map(container net.Addr, hostIP net.IP, hostPort int) error {
	lock.Lock()
	defer lock.Unlock()

	var m *mapping
	switch container.(type) {
	case *net.TCPAddr:
		m = &mapping{
			proto:     "tcp",
			host:      &net.TCPAddr{IP: hostIP, Port: hostPort},
			container: container,
		}
	case *net.UDPAddr:
		m = &mapping{
			proto:     "udp",
			host:      &net.UDPAddr{IP: hostIP, Port: hostPort},
			container: container,
		}
	default:
		return ErrUnknownBackendAddressType
	}

	key := getKey(m.host)
	if _, exists := currentMappings[key]; exists {
		return ErrPortMappedForIP
	}

	containerIP, containerPort := getIPAndPort(m.container)
	if err := forward(iptables.Add, m.proto, hostIP, hostPort, containerIP.String(), containerPort); err != nil {
		return err
	}

	p, err := newProxy(m.host, m.container)
	if err != nil {
		// need to undo the iptables rules before we reutrn
		forward(iptables.Delete, m.proto, hostIP, hostPort, containerIP.String(), containerPort)
		return err
	}

	m.userlandProxy = p
	currentMappings[key] = m

	go p.Run()

	return nil
}

func Unmap(host net.Addr) error {
	lock.Lock()
	defer lock.Unlock()

	key := getKey(host)
	data, exists := currentMappings[key]
	if !exists {
		return ErrPortNotMapped
	}

	data.userlandProxy.Close()
	delete(currentMappings, key)

	containerIP, containerPort := getIPAndPort(data.container)
	hostIP, hostPort := getIPAndPort(data.host)
	if err := forward(iptables.Delete, data.proto, hostIP, hostPort, containerIP.String(), containerPort); err != nil {
		return err
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
