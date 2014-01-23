package portallocator

import (
	"errors"
	"github.com/dotcloud/docker/pkg/collections"
	"sync"
)

type portMappings map[string]*collections.OrderedIntSet

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
	lock           = sync.Mutex{}
	allocatedPorts = portMappings{}
	availablePorts = portMappings{}
)

func init() {
	allocatedPorts["udp"] = collections.NewOrderedIntSet()
	availablePorts["udp"] = collections.NewOrderedIntSet()
	allocatedPorts["tcp"] = collections.NewOrderedIntSet()
	availablePorts["tcp"] = collections.NewOrderedIntSet()
}

// RequestPort returns an available port if the port is 0
// If the provided port is not 0 then it will be checked if
// it is available for allocation
func RequestPort(proto string, port int) (int, error) {
	lock.Lock()
	defer lock.Unlock()

	if err := validateProtocol(proto); err != nil {
		return 0, err
	}

	var (
		allocated = allocatedPorts[proto]
		available = availablePorts[proto]
	)

	if port != 0 {
		if allocated.Exists(port) {
			return 0, ErrPortAlreadyAllocated
		}
		available.Remove(port)
		allocated.Push(port)
		return port, nil
	}

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
func ReleasePort(proto string, port int) error {
	lock.Lock()
	defer lock.Unlock()

	if err := validateProtocol(proto); err != nil {
		return err
	}

	var (
		allocated = allocatedPorts[proto]
		available = availablePorts[proto]
	)

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
