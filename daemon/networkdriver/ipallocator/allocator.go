package ipallocator

import (
	"encoding/binary"
	"errors"
	"net"
	"sync"

	"github.com/docker/docker/daemon/networkdriver"
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
		begin: begin,
		end:   end,
		last:  begin - 1, // so first allocated will be begin
	}
}

type networkSet map[string]*allocatedMap

var (
	ErrNoAvailableIPs           = errors.New("no available ip addresses on network")
	ErrIPAlreadyAllocated       = errors.New("ip already allocated")
	ErrNetworkAlreadyRegistered = errors.New("network already registered")
	ErrBadSubnet                = errors.New("network not contains specified subnet")
)

var (
	lock         = sync.Mutex{}
	allocatedIPs = networkSet{}
)

// RegisterSubnet registers network in global allocator with bounds
// defined by subnet. If you want to use network range you must call
// this method before first RequestIP, otherwise full network range will be used
func RegisterSubnet(network *net.IPNet, subnet *net.IPNet) error {
	lock.Lock()
	defer lock.Unlock()
	key := network.String()
	if _, ok := allocatedIPs[key]; ok {
		return ErrNetworkAlreadyRegistered
	}
	n := newAllocatedMap(network)
	beginIP, endIP := networkdriver.NetworkRange(subnet)
	begin, end := ipToInt(beginIP)+1, ipToInt(endIP)-1
	if !(begin >= n.begin && end <= n.end && begin < end) {
		return ErrBadSubnet
	}
	n.begin = begin
	n.end = end
	n.last = begin - 1
	allocatedIPs[key] = n
	return nil
}

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
