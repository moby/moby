package portallocator

import (
	"errors"
	"fmt"
	"net"
	"sync"
)

type portMap struct {
	p    map[int]struct{}
	last int
}

type (
	protocolMap map[string]*portMap
	ipMapping   map[string]protocolMap
)

const (
	BeginPortRange = 49153
	EndPortRange   = 65535
)

var (
	ErrAllPortsAllocated = errors.New("all ports are allocated")
	ErrUnknownProtocol   = errors.New("unknown protocol")
)

var (
	mutex sync.Mutex

	defaultIP = net.ParseIP("0.0.0.0")
	globalMap = ipMapping{}
)

type ErrPortAlreadyAllocated struct {
	ip   string
	port int
}

func NewErrPortAlreadyAllocated(ip string, port int) ErrPortAlreadyAllocated {
	return ErrPortAlreadyAllocated{
		ip:   ip,
		port: port,
	}
}

func (e ErrPortAlreadyAllocated) IP() string {
	return e.ip
}

func (e ErrPortAlreadyAllocated) Port() int {
	return e.port
}

func (e ErrPortAlreadyAllocated) IPPort() string {
	return fmt.Sprintf("%s:%d", e.ip, e.port)
}

func (e ErrPortAlreadyAllocated) Error() string {
	return fmt.Sprintf("Bind for %s:%d failed: port is already allocated", e.ip, e.port)
}

func RequestPort(ip net.IP, proto string, port int) (int, error) {
	mutex.Lock()
	defer mutex.Unlock()

	if err := validateProto(proto); err != nil {
		return 0, err
	}

	ip = getDefault(ip)

	mapping := getOrCreate(ip)

	if port > 0 {
		if _, ok := mapping[proto].p[port]; !ok {
			mapping[proto].p[port] = struct{}{}
			return port, nil
		} else {
			return 0, NewErrPortAlreadyAllocated(ip.String(), port)
		}
	} else {
		port, err := findPort(ip, proto)

		if err != nil {
			return 0, err
		}

		return port, nil
	}
}

func ReleasePort(ip net.IP, proto string, port int) error {
	mutex.Lock()
	defer mutex.Unlock()

	ip = getDefault(ip)

	mapping := getOrCreate(ip)[proto]
	delete(mapping.p, port)

	return nil
}

func ReleaseAll() error {
	mutex.Lock()
	defer mutex.Unlock()

	globalMap = ipMapping{}

	return nil
}

func getOrCreate(ip net.IP) protocolMap {
	ipstr := ip.String()

	if _, ok := globalMap[ipstr]; !ok {
		globalMap[ipstr] = protocolMap{
			"tcp": &portMap{p: map[int]struct{}{}, last: 0},
			"udp": &portMap{p: map[int]struct{}{}, last: 0},
		}
	}

	return globalMap[ipstr]
}

func findPort(ip net.IP, proto string) (int, error) {
	mapping := getOrCreate(ip)[proto]

	if mapping.last == 0 {
		mapping.p[BeginPortRange] = struct{}{}
		mapping.last = BeginPortRange
		return BeginPortRange, nil
	}

	for port := mapping.last + 1; port != mapping.last; port++ {
		if port > EndPortRange {
			port = BeginPortRange
		}

		if _, ok := mapping.p[port]; !ok {
			mapping.p[port] = struct{}{}
			mapping.last = port
			return port, nil
		}

	}

	return 0, ErrAllPortsAllocated
}

func getDefault(ip net.IP) net.IP {
	if ip == nil {
		return defaultIP
	}

	return ip
}

func validateProto(proto string) error {
	if proto != "tcp" && proto != "udp" {
		return ErrUnknownProtocol
	}

	return nil
}
