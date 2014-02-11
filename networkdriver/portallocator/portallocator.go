package portallocator

import (
	"errors"
	"github.com/dotcloud/docker/pkg/collections"
	"net"
	"sync"
)

const (
	BeginPortRange = 49153
	EndPortRange   = 65535
)

type (
	portMappings map[string]*collections.OrderedIntSet
	ipMapping    map[string]portMappings
)

var (
	ErrPortAlreadyAllocated = errors.New("port has already been allocated")
	ErrPortExceedsRange     = errors.New("port exceeds upper range")
	ErrUnknownProtocol      = errors.New("unknown protocol")
)

var (
	currentDynamicPort = map[string]int{
		"tcp": BeginPortRange - 1,
		"udp": BeginPortRange - 1,
	}
	defaultIP             = net.ParseIP("0.0.0.0")
	defaultAllocatedPorts = portMappings{}
	otherAllocatedPorts   = ipMapping{}
	lock                  = sync.Mutex{}
)

func init() {
	defaultAllocatedPorts["tcp"] = collections.NewOrderedIntSet()
	defaultAllocatedPorts["udp"] = collections.NewOrderedIntSet()
}

// RequestPort returns an available port if the port is 0
// If the provided port is not 0 then it will be checked if
// it is available for allocation
func RequestPort(ip net.IP, proto string, port int) (int, error) {
	lock.Lock()
	defer lock.Unlock()

	if err := validateProtocol(proto); err != nil {
		return 0, err
	}

	// If the user requested a specific port to be allocated
	if port > 0 {
		if err := registerSetPort(ip, proto, port); err != nil {
			return 0, err
		}
		return port, nil
	}
	return registerDynamicPort(ip, proto)
}

// ReleasePort will return the provided port back into the
// pool for reuse
func ReleasePort(ip net.IP, proto string, port int) error {
	lock.Lock()
	defer lock.Unlock()

	if err := validateProtocol(proto); err != nil {
		return err
	}

	allocated := defaultAllocatedPorts[proto]
	allocated.Remove(port)

	if !equalsDefault(ip) {
		registerIP(ip)

		// Remove the port for the specific ip address
		allocated = otherAllocatedPorts[ip.String()][proto]
		allocated.Remove(port)
	}
	return nil
}

func ReleaseAll() error {
	lock.Lock()
	defer lock.Unlock()

	currentDynamicPort["tcp"] = BeginPortRange - 1
	currentDynamicPort["udp"] = BeginPortRange - 1

	defaultAllocatedPorts = portMappings{}
	defaultAllocatedPorts["tcp"] = collections.NewOrderedIntSet()
	defaultAllocatedPorts["udp"] = collections.NewOrderedIntSet()

	otherAllocatedPorts = ipMapping{}

	return nil
}

func registerDynamicPort(ip net.IP, proto string) (int, error) {
	allocated := defaultAllocatedPorts[proto]

	port := nextPort(proto)
	if port > EndPortRange {
		return 0, ErrPortExceedsRange
	}

	if !equalsDefault(ip) {
		registerIP(ip)

		ipAllocated := otherAllocatedPorts[ip.String()][proto]
		ipAllocated.Push(port)
	} else {
		allocated.Push(port)
	}
	return port, nil
}

func registerSetPort(ip net.IP, proto string, port int) error {
	allocated := defaultAllocatedPorts[proto]
	if allocated.Exists(port) {
		return ErrPortAlreadyAllocated
	}

	if !equalsDefault(ip) {
		registerIP(ip)

		ipAllocated := otherAllocatedPorts[ip.String()][proto]
		if ipAllocated.Exists(port) {
			return ErrPortAlreadyAllocated
		}
		ipAllocated.Push(port)
	} else {
		allocated.Push(port)
	}
	return nil
}

func equalsDefault(ip net.IP) bool {
	return ip == nil || ip.Equal(defaultIP)
}

func nextPort(proto string) int {
	c := currentDynamicPort[proto] + 1
	currentDynamicPort[proto] = c
	return c
}

func registerIP(ip net.IP) {
	if _, exists := otherAllocatedPorts[ip.String()]; !exists {
		otherAllocatedPorts[ip.String()] = portMappings{
			"tcp": collections.NewOrderedIntSet(),
			"udp": collections.NewOrderedIntSet(),
		}
	}
}

func validateProtocol(proto string) error {
	if _, exists := defaultAllocatedPorts[proto]; !exists {
		return ErrUnknownProtocol
	}
	return nil
}
