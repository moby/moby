package ipallocator

import (
	"encoding/binary"
	"errors"
	"github.com/dotcloud/docker/utils"
	"github.com/dotcloud/docker/networkdriver"
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
		if !utils.IsIPv6(&address.IP) {
			next, err := getNextIp(address)
			if err != nil {
				return nil, err
			}
			return next, nil
		} else {
			next, err := getNextIp6(address)
			if err != nil {
				return nil, err
			}
			return next, nil
		}
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
		pos uint64
	)

	if !utils.IsIPv6(ip) {
		pos = uint64(getPosition(address, ip))
	} else {
		_, pos = getPosition6(address, ip)
	}
	existing.Remove(pos)
	available.Push(pos)

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

func getPosition6(address *net.IPNet, ip *net.IP) (uint64, uint64) {
	var (
		first, _    = networkdriver.NetworkRange(address)
		base, base2 = ip6ToInt(&first)
		i, i2	    = ip6ToInt(ip)
	)
	return i - base, i2 - base2
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
		max       = uint64(size - 2) // size -1 for the broadcast address, -1 for the gateway address
		pos       = available.Pop()
	)


	// We pop and push the position not the ip
	if pos != 0 {
		ip := intToIP(base + int32(pos))
		allocated.Push(pos)

		return ip, nil
	}

	var (
		firstNetIP = address.IP.To4().Mask(address.Mask)
		firstAsInt = ipToInt(&firstNetIP) + 1
	)

	pos = allocated.PullBack()
	for i := uint64(0); i < max; i++ {
		pos = pos%max + 1
		next := base + int32(pos)

		if next == ownIP || next == firstAsInt {
			continue
		}

		if !allocated.Exists(pos) {
			ip := intToIP(next)
			allocated.Push(pos)
			return ip, nil
		}
	}
	return nil, ErrNoAvailableIPs
}

// return an available ip if one is currently available.  If not,
// return the next available ip for the nextwork
func getNextIp6(address *net.IPNet) (*net.IP, error) {
	var (
		_, ownIP      = ip6ToInt(&address.IP)
		available     = availableIPS[address.String()]
		allocated     = allocatedIPs[address.String()]
		first, _      = networkdriver.NetworkRange(address)
		baseTop, base = ip6ToInt(&first)
		_, max        = networkdriver.NetworkSize6(address.Mask)
		pos           = available.Pop()
	)

	// We pop and push the position not the ip
	if pos != 0 {
		ip := intToIP6(baseTop, base + pos)
		allocated.Push(pos)

		return ip, nil
	}

	var (
		firstNetIP = address.IP.To16().Mask(address.Mask)
		_, firstAsInt = ip6ToInt(&firstNetIP)
	)
	firstAsInt = firstAsInt + 1

	pos = allocated.PullBack()
	for i := uint64(0); i < max; i++ {
		pos = pos%max + 1
		next := base + pos

		if next == ownIP || next == firstAsInt {
			continue
		}

		if !allocated.Exists(pos) {
			ip := intToIP6(baseTop, next)
			allocated.Push(pos)
			return ip, nil
		}
	}
	return nil, ErrNoAvailableIPs
}

func registerIP(address *net.IPNet, ip *net.IP) error {
	var (
		existing  = allocatedIPs[address.String()]
		available = availableIPS[address.String()]
		pos uint64
	)

	if !utils.IsIPv6(ip) {
		pos = uint64(getPosition(address, ip))
	} else {
		_, pos = getPosition6(address, ip)
	}

	if existing.Exists(pos) {
		return ErrIPAlreadyAllocated
	}
	available.Remove(pos)

	return nil
}

// Converts a 4 bytes IP into a 32 bit integer
func ipToInt(ip *net.IP) int32 {
	return int32(binary.BigEndian.Uint32(ip.To4()))
}

// Converts a 16 byte IP into two 64-bit integers
func ip6ToInt(ip *net.IP) (uint64, uint64) {
	b := make([]byte, 8)
	b2 := make([]byte, 8)
	ip2 := ip.To16()

	for i := 0; i < len(b); i++ {
		n := i + 8
		b[i] = ip2[i]
		b2[i] = ip2[n]
	}
	return binary.BigEndian.Uint64(b), binary.BigEndian.Uint64(b2)
}

// Converts 32 bit integer into a 4 bytes IP address
func intToIP(n int32) *net.IP {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(n))
	ip := net.IP(b)
	return &ip
}

// Converts 2 64 bit integers into a 16 byte IP address
func intToIP6(n1 uint64, n2 uint64) *net.IP {
	b  := make([]byte, 8)
	b2 := make([]byte, 8)
	final := make([]byte, 16)

	binary.BigEndian.PutUint64(b, n1)
	binary.BigEndian.PutUint64(b2, n2)

	for i := 0; i < len(b); i++ {
		n := i + 8
		final[i] = b[i]
		final[n] = b2[i]
	}

	ip := net.IP(final)
	return &ip
}

func checkAddress(address *net.IPNet) {
	key := address.String()
	if _, exists := allocatedIPs[key]; !exists {
		allocatedIPs[key] = collections.NewOrderedIntSet()
		availableIPS[key] = collections.NewOrderedIntSet()
	}
}
