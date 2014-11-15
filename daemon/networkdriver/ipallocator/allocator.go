package ipallocator

import (
	"errors"
	"math/big"
	"net"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/networkdriver"
	"github.com/docker/docker/daemon/networkdriver/allocator"
)

func netRange(network *net.IPNet) (*big.Int, *big.Int) {
	firstIP, lastIP := networkdriver.NetworkRange(network)
	begin := big.NewInt(0).Add(ipToBigInt(firstIP), big.NewInt(1))
	end := big.NewInt(0).Sub(ipToBigInt(lastIP), big.NewInt(1))
	return begin, end
}

func newAllocator(network *net.IPNet) *allocator.Allocator {
	begin, end := netRange(network)
	return allocator.NewAllocator(begin, end)
}

type networkSet map[string]*allocator.Allocator

var (
	ErrNoAvailableIPs           = errors.New("no available ip addresses on network")
	ErrIPAlreadyAllocated       = errors.New("ip already allocated")
	ErrIPOutOfRange             = errors.New("requested ip is out of range")
	ErrNetworkAlreadyRegistered = errors.New("network already registered")
	ErrBadSubnet                = errors.New("network does not contain specified subnet")
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

	// if IPv4 network, then allocation range starts at begin + 1 because begin is bridge IP
	begin, end := netRange(network)
	if len(begin.Bytes()) == 4 {
		begin = begin.Add(begin, big.NewInt(1))
	}

	// Check that subnet is within network
	subBegin, subEnd := netRange(subnet)
	if !(subBegin.Cmp(begin) >= 0 && subEnd.Cmp(end) <= 0 && subBegin.Cmp(subEnd) == -1) {
		return ErrBadSubnet
	}

	allocatedIPs[key] = newAllocator(subnet)
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
		// if IPv4 network, then allocation range starts at begin + 1 because begin is bridge IP
		begin, end := netRange(network)
		if len(begin.Bytes()) == 4 {
			begin = begin.Add(begin, big.NewInt(1))
		}
		allocated = allocator.NewAllocator(begin, end)
		allocatedIPs[key] = allocated
	}

	if ip == nil {
		next, err := allocated.AllocateFirstAvailable()
		if err != nil {
			return nil, ErrNoAvailableIPs
		}
		return bigIntToIP(next), nil
	}

	if err := allocated.Allocate(ipToBigInt(ip)); err != nil {
		return nil, ErrIPAlreadyAllocated
	}

	pos := ipToBigInt(ip)
	// Verify that the IP address is within our network range.
	begin, end := netRange(network)
	if pos.Cmp(begin) == -1 || pos.Cmp(end) == 1 {
		return nil, ErrIPOutOfRange
	}

	return ip, nil
}

// ReleaseIP adds the provided ip back into the pool of
// available ips to be returned for use.
func ReleaseIP(network *net.IPNet, ip net.IP) error {
	lock.Lock()
	defer lock.Unlock()
	if allocated, exists := allocatedIPs[network.String()]; exists {
		allocated.Release(ipToBigInt(ip))
	}
	return nil
}

// Converts a 4 bytes IP into a 128 bit integer
func ipToBigInt(ip net.IP) *big.Int {
	x := big.NewInt(0)
	if ip4 := ip.To4(); ip4 != nil {
		return x.SetBytes(ip4)
	}
	if ip6 := ip.To16(); ip6 != nil {
		return x.SetBytes(ip6)
	}

	log.Errorf("ipToBigInt: Wrong IP length! %s", ip)
	return nil
}

// Converts 128 bit integer into a 4 bytes IP address
func bigIntToIP(v *big.Int) net.IP {
	return net.IP(v.Bytes())
}
