package ipallocator

import (
	"encoding/binary"
	"errors"
	"net"
	"sync"

	"github.com/dotcloud/docker/daemon/networkdriver"
)

// allocatedMap is thread-unsafe set of allocated IP
type allocatedMap struct {
	p     map[uint32]struct{}
	last  uint32
	begin uint32
	end   uint32
}

func newAllocatedMap(network *net.IPNet) *allocatedMap {
	firstIP, lastIP := networkdriver.NetworkRange(network)
	begin := ipToInt(firstIP) + 2
	end := ipToInt(lastIP) - 1
	return &allocatedMap{
		p:     make(map[uint32]struct{}),
		begin: begin,     // - network
		end:   end,       // - broadcast
		last:  begin - 1, // so first allocated will be begin
	}
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
func RequestIP(network *net.IPNet, ip net.IP) (net.IP, error) {
	lock.Lock()
	defer lock.Unlock()
	key := network.String()
	allocated, ok := allocatedIPs[key]
	if !ok {
		allocated = newAllocatedMap(network)
		allocatedIPs[key] = allocated
	}

	if ip == nil {
		return allocated.getNextIP()
	}
	return allocated.checkIP(ip)
}

// ReleaseIP adds the provided ip back into the pool of
// available ips to be returned for use.
func ReleaseIP(network *net.IPNet, ip net.IP) error {
	lock.Lock()
	defer lock.Unlock()
	if allocated, exists := allocatedIPs[network.String()]; exists {
		pos := ipToInt(ip)
		delete(allocated.p, pos)
	}
	return nil
}

func (allocated *allocatedMap) checkIP(ip net.IP) (net.IP, error) {
	pos := ipToInt(ip)
	if _, ok := allocated.p[pos]; ok {
		return nil, ErrIPAlreadyAllocated
	}
	allocated.p[pos] = struct{}{}
	allocated.last = pos
	return ip, nil
}

// return an available ip if one is currently available.  If not,
// return the next available ip for the nextwork
func (allocated *allocatedMap) getNextIP() (net.IP, error) {
	for pos := allocated.last + 1; pos != allocated.last; pos++ {
		if pos > allocated.end {
			pos = allocated.begin
		}
		if _, ok := allocated.p[pos]; ok {
			continue
		}
		allocated.p[pos] = struct{}{}
		allocated.last = pos
		return intToIP(pos), nil
	}
	return nil, ErrNoAvailableIPs
}

// Converts a 4 bytes IP into a 32 bit integer
func ipToInt(ip net.IP) uint32 {
	return binary.BigEndian.Uint32(ip.To4())
}

// Converts 32 bit integer into a 4 bytes IP address
func intToIP(n uint32) net.IP {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, n)
	ip := net.IP(b)
	return ip
}
