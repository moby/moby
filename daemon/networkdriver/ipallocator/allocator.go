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
func RequestIP(network *net.IPNet, ipRange *net.IPNet) (*net.IP, error) {
	lock.Lock()
	defer lock.Unlock()
	key := network.String()
	allocated, ok := allocatedIPs[key]
	if !ok {
		allocated = newAllocatedMap()
		allocatedIPs[key] = allocated
	}

	return allocated.getNextIP(network, ipRange)
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

// return an available ip if one is currently available.
func (allocated *allocatedMap) getNextIP(network *net.IPNet, ipReqRange *net.IPNet) (*net.IP, error) {
	var (
		ownIPInt           = ipToInt(&network.IP)
		base, broadcast    = networkdriver.NetworkRange(network)
		baseInt            = ipToInt(&base)
		networkFirstInt    = baseInt + 1
		networkLastInt     = ipToInt(&broadcast) - 1
		rangeFirstInt      = networkFirstInt
		rangeLastInt       = networkLastInt
		subnetAllocatedPos = allocated.last
		pos                = subnetAllocatedPos - (rangeFirstInt - baseInt) // relative to the start of available addresses
	)

	if ipReqRange != nil {
		ipReqRangeFirst, ipReqRangeLast := networkdriver.NetworkRange(ipReqRange)
		ipReqRangeFirstInt := ipToInt(&ipReqRangeFirst)
		ipReqRangeLastInt := ipToInt(&ipReqRangeLast)

		if rangeFirstInt < ipReqRangeFirstInt {
			rangeFirstInt = ipReqRangeFirstInt
		}
		if rangeLastInt > ipReqRangeLastInt {
			rangeLastInt = ipReqRangeLastInt
		}

		pos = pos - (ipReqRangeFirstInt - networkFirstInt) // make relative to the new range
	}

	size := rangeLastInt - rangeFirstInt + 1
	if pos < 0 || pos > size { // outside of range
		pos = 0
	}
	for i := int32(0); i < size; i++ {
		pos = (pos + 1) % size
		ipInt := int32(rangeFirstInt + pos)

		if ipInt == ownIPInt {
			continue
		}
		allocatedPos := pos + (rangeFirstInt - baseInt)
		if _, ok := allocated.p[allocatedPos]; ok {
			continue
		}
		allocated.p[allocatedPos] = struct{}{}
		allocated.last = allocatedPos
		return intToIP(ipInt), nil
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
