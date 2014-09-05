package ipallocator

import (
	"encoding/binary"
	"errors"
	"github.com/docker/docker/daemon/networkdriver"
	"net"
	"sync"
)

// allocatedMap is thread-unsafe set of allocated IP
type allocatedMap struct {
	p    map[int32]struct{}
	last int32
}

func newAllocatedMap() *allocatedMap {
	return &allocatedMap{p: make(map[int32]struct{})}
}

type networkSet map[string]*allocatedMap

var (
	ErrNoAvailableIPs     = errors.New("no available ip addresses on network")
	ErrIPAlreadyAllocated = errors.New("ip already allocated")
)

var (
	lock         = sync.Mutex{}
	allocatedIPs = networkSet{}
)

// RequestIP requests an available ip from the given network.  It
// will return the next available ip if the ip provided is nil.  If the
// ip provided is not nil it will validate that the provided ip is available
// for use or return an error
func RequestIP(network *net.IPNet, ip *net.IP) (*net.IP, error) {
	lock.Lock()
	defer lock.Unlock()
	key := network.String()
	allocated, ok := allocatedIPs[key]
	if !ok {
		allocated = newAllocatedMap()
		allocatedIPs[key] = allocated
	}

	if ip == nil {
		return allocated.getNextIP(network)
	}
	return allocated.checkIP(network, ip)
}

// ReleaseIP adds the provided ip back into the pool of
// available ips to be returned for use.
func ReleaseIP(network *net.IPNet, ip *net.IP) error {
	lock.Lock()
	defer lock.Unlock()
	if allocated, exists := allocatedIPs[network.String()]; exists {
		pos := getPosition(network, ip)
		delete(allocated.p, pos)
	}
	return nil
}

// convert the ip into the position in the subnet.  Only
// position are saved in the set
func getPosition(network *net.IPNet, ip *net.IP) int32 {
	first, _ := networkdriver.NetworkRange(network)
	return ipToInt(ip) - ipToInt(&first)
}

func (allocated *allocatedMap) checkIP(network *net.IPNet, ip *net.IP) (*net.IP, error) {
	pos := getPosition(network, ip)
	if _, ok := allocated.p[pos]; ok {
		return nil, ErrIPAlreadyAllocated
	}
	allocated.p[pos] = struct{}{}
	allocated.last = pos
	return ip, nil
}

// return an available ip if one is currently available.  If not,
// return the next available ip for the nextwork
func (allocated *allocatedMap) getNextIP(network *net.IPNet) (*net.IP, error) {
	var (
		ownIP    = ipToInt(&network.IP)
		first, _ = networkdriver.NetworkRange(network)
		base     = ipToInt(&first)
		size     = int(networkdriver.NetworkSize(network.Mask))
		max      = int32(size - 2) // size -1 for the broadcast network, -1 for the gateway network
		pos      = allocated.last
	)

	var (
		firstNetIP = network.IP.To4().Mask(network.Mask)
		firstAsInt = ipToInt(&firstNetIP) + 1
	)

	for i := int32(0); i < max; i++ {
		pos = pos%max + 1
		next := int32(base + pos)

		if next == ownIP || next == firstAsInt {
			continue
		}
		if _, ok := allocated.p[pos]; ok {
			continue
		}
		allocated.p[pos] = struct{}{}
		allocated.last = pos
		return intToIP(next), nil
	}
	return nil, ErrNoAvailableIPs
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
