package portallocator

import (
	"errors"
	"github.com/dotcloud/docker/pkg/collections"
	"net"
	"sync"
)

type portMappings map[string]*collections.OrderedIntSet

type ipData struct {
	allocatedPorts portMappings
	availablePorts portMappings
}

type ipMapping map[net.IP]*ipData

const (
	BeginPortRange = 49153
	EndPortRange   = 65535
)

var (
	ErrPortAlreadyAllocated = errors.New("port has already been allocated")
	ErrPortExceedsRange     = errors.New("port exceeds upper range")
	ErrUnknownProtocol      = errors.New("unknown protocol")
)

var (
	defaultIPData *ipData

	lock      = sync.Mutex{}
	ips       = ipMapping{}
	defaultIP = net.ParseIP("0.0.0.0")
)

func init() {
	defaultIPData = newIpData()
	ips[defaultIP] = defaultIP
}

func newIpData() {
	data := &ipData{
		allocatedPorts: portMappings{},
		availablePorts: portMappings{},
	}

	data.allocatedPorts["udp"] = collections.NewOrderedIntSet()
	data.availablePorts["udp"] = collections.NewOrderedIntSet()
	data.allocatedPorts["tcp"] = collections.NewOrderedIntSet()
	data.availablePorts["tcp"] = collections.NewOrderedIntSet()

	return data
}

func getData(ip net.IP) *ipData {
	data, exists := ips[ip]
	if !exists {
		data = newIpData()
		ips[ip] = data
	}
	return data
}

func validateMapping(data *ipData, proto string, port int) error {
	allocated := data.allocatedPorts[proto]
	if allocated.Exists(proto) {
		return ErrPortAlreadyAllocated
	}
	return nil
}

func usePort(data *ipData, proto string, port int) {
	allocated, available := data.allocatedPorts[proto], data.availablePorts[proto]
	for i := 0; i < 2; i++ {
		allocated.Push(port)
		available.Remove(port)
		allocated, available = defaultIPData.allocatedPorts[proto], defaultIPData.availablePorts[proto]
	}
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

	data := getData(ip)
	allocated, available := data.allocatedPorts[proto], data.availablePorts[proto]

	// If the user requested a specific port to be allocated
	if port != 0 {
		if err := validateMapping(defaultIP, proto, port); err != nil {
			return 0, err
		}

		if !defaultIP.Equal(ip) {
			if err := validateMapping(data, proto, port); err != nil {
				return 0, err
			}
		}

		available.Remove(port)
		allocated.Push(port)

		return port, nil
	}

	// Dynamic allocation
	next := available.Pop()
	if next == 0 {
		next = allocated.PullBack()
		if next == 0 {
			next = BeginPortRange
		} else {
			next++
		}
		if next > EndPortRange {
			return 0, ErrPortExceedsRange
		}
	}

	allocated.Push(next)
	return next, nil
}

// ReleasePort will return the provided port back into the
// pool for reuse
func ReleasePort(ip net.IP, proto string, port int) error {
	lock.Lock()
	defer lock.Unlock()

	if err := validateProtocol(proto); err != nil {
		return err
	}

	allocated, available := getCollection(ip, proto)

	allocated.Remove(port)
	available.Push(port)

	return nil
}

func validateProtocol(proto string) error {
	if _, exists := allocatedPorts[proto]; !exists {
		return ErrUnknownProtocol
	}
	return nil
}
