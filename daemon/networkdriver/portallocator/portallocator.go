package portallocator

import (
	"errors"
	"net"
	"sync"
)

type (
	portMap     map[int]bool
	protocolMap map[string]portMap
	ipMapping   map[string]protocolMap
)

const (
	BeginPortRange = 49153
	EndPortRange   = 65535
)

var (
	ErrAllPortsAllocated    = errors.New("all ports are allocated")
	ErrPortAlreadyAllocated = errors.New("port has already been allocated")
	ErrUnknownProtocol      = errors.New("unknown protocol")
)

var (
	mutex sync.Mutex

	defaultIP = net.ParseIP("0.0.0.0")
	globalMap = ipMapping{}
)

func RequestPort(ip net.IP, proto string, port int) (int, error) {
	mutex.Lock()
	defer mutex.Unlock()

	if err := validateProto(proto); err != nil {
		return 0, err
	}

	ip = getDefault(ip)

	mapping := getOrCreate(ip)

	if port > 0 {
		if !mapping[proto][port] {
			mapping[proto][port] = true
			return port, nil
		} else {
			return 0, ErrPortAlreadyAllocated
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

	mapping := getOrCreate(ip)
	delete(mapping[proto], port)

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
			"tcp": portMap{},
			"udp": portMap{},
		}
	}

	return globalMap[ipstr]
}

func findPort(ip net.IP, proto string) (int, error) {
	port := BeginPortRange

	mapping := getOrCreate(ip)

	for mapping[proto][port] {
		port++

		if port > EndPortRange {
			return 0, ErrAllPortsAllocated
		}
	}

	mapping[proto][port] = true

	return port, nil
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
