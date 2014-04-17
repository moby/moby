package ipallocator

import (
	"encoding/binary"
	"errors"
	"github.com/dotcloud/docker/daemon/networkdriver"
	"github.com/dotcloud/docker/pkg/collections"
	"net"
	"sync"
)

type networkSet map[string]*collections.OrderedIntSet

var (
	ErrNoAvailableIPs     = errors.New("no available ip addresses on network")
	ErrIPAlreadyAllocated = errors.New("ip already allocated")
)

var (
	lock         = sync.Mutex{}
	allocatedIPs = networkSet{}
	availableIPS = networkSet{}
)

// RequestIP requests an available ip from the given network.  It
// will return the next available ip if the ip provided is nil.  If the
// ip provided is not nil it will validate that the provided ip is available
// for use or return an error
func RequestIP(address *net.IPNet, ip *net.IP) (*net.IP, error) {
	lock.Lock()
	defer lock.Unlock()

	checkAddress(address)

	if ip == nil {
		next, err := getNextIp(address)
		if err != nil {
			return nil, err
		}
		return next, nil
	}

	if err := registerIP(address, ip); err != nil {
		return nil, err
	}
	return ip, nil
}

// ReleaseIP adds the provided ip back into the pool of
// available ips to be returned for use.
func ReleaseIP(address *net.IPNet, ip *net.IP) error {
	lock.Lock()
	defer lock.Unlock()

	checkAddress(address)

	var (
		existing  = allocatedIPs[address.String()]
		available = availableIPS[address.String()]
		pos       = getPosition(address, ip)
	)

	existing.Remove(int(pos))
	available.Push(int(pos))

	return nil
}

// convert the ip into the position in the subnet.  Only
// position are saved in the set
func getPosition(address *net.IPNet, ip *net.IP) int32 {
	var (
		first, _ = networkdriver.NetworkRange(address)
		base     = ipToInt(&first)
		i        = ipToInt(ip)
	)
	return i - base
}

// return an available ip if one is currently available.  If not,
// return the next available ip for the nextwork
func getNextIp(address *net.IPNet) (*net.IP, error) {
	var (
		ownIP     = ipToInt(&address.IP)
		available = availableIPS[address.String()]
		allocated = allocatedIPs[address.String()]
		first, _  = networkdriver.NetworkRange(address)
		base      = ipToInt(&first)
		size      = int(networkdriver.NetworkSize(address.Mask))
		max       = int32(size - 2) // size -1 for the broadcast address, -1 for the gateway address
		pos       = int32(available.Pop())
	)

	// We pop and push the position not the ip
	if pos != 0 {
		ip := intToIP(int32(base + pos))
		allocated.Push(int(pos))

		return ip, nil
	}

	var (
		firstNetIP = address.IP.To4().Mask(address.Mask)
		firstAsInt = ipToInt(&firstNetIP) + 1
	)

	pos = int32(allocated.PullBack())
	for i := int32(0); i < max; i++ {
		pos = pos%max + 1
		next := int32(base + pos)

		if next == ownIP || next == firstAsInt {
			continue
		}

		if !allocated.Exists(int(pos)) {
			ip := intToIP(next)
			allocated.Push(int(pos))
			return ip, nil
		}
	}
	return nil, ErrNoAvailableIPs
}

func registerIP(address *net.IPNet, ip *net.IP) error {
	var (
		existing  = allocatedIPs[address.String()]
		available = availableIPS[address.String()]
		pos       = getPosition(address, ip)
	)

	if existing.Exists(int(pos)) {
		return ErrIPAlreadyAllocated
	}
	available.Remove(int(pos))

	return nil
}

// Converts a 4 bytes IP into a 32 bit integer
func ipToInt(ip *net.IP) int32 {
	return int32(binary.BigEndian.Uint32(ip.To4()))
}

// Converts 32 bit integer into a 4 bytes IP address
func intToIP(n int32) *net.IP {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(n))
	ip := net.IP(b)
	return &ip
}

func checkAddress(address *net.IPNet) {
	key := address.String()
	if _, exists := allocatedIPs[key]; !exists {
		allocatedIPs[key] = collections.NewOrderedIntSet()
		availableIPS[key] = collections.NewOrderedIntSet()
	}
}
