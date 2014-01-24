package ipallocator

import (
	"encoding/binary"
	"errors"
	"github.com/dotcloud/docker/pkg/netlink"
	"net"
	"sync"
)

type networkSet map[iPNet]*iPSet

type iPNet struct {
	IP   string
	Mask string
}

var (
	ErrNetworkAlreadyAllocated        = errors.New("requested network overlaps with existing network")
	ErrNetworkAlreadyRegisterd        = errors.New("requested network is already registered")
	ErrNetworkOverlapsWithNameservers = errors.New("requested network overlaps with nameserver")
	ErrNoAvailableIPs                 = errors.New("no available ip addresses on network")
	ErrIPAlreadyAllocated             = errors.New("ip already allocated")
	ErrNetworkNotRegistered           = errors.New("network not registered")

	lock         = sync.Mutex{}
	allocatedIPs = networkSet{}
	availableIPS = networkSet{}
)

// RegisterNetwork registers a new network with the allocator
// and validates that it contains a valid ip that does not overlap
// with existing routes and nameservers
func RegisterNetwork(network *net.IPNet, nameservers []string) error {
	lock.Lock()
	defer lock.Unlock()

	if err := checkExistingNetworkOverlaps(network); err != nil {
		return err
	}

	routes, err := netlink.NetworkGetRoutes()
	if err != nil {
		return err
	}

	if err := checkRouteOverlaps(routes, network); err != nil {
		return err
	}

	if err := checkNameserverOverlaps(nameservers, network); err != nil {
		return err
	}
	return RegisterExistingNetwork(network)
}

// RegisterExistingNetwork registers an exising network created
// for use with the allocator but does not perform any validation
func RegisterExistingNetwork(network *net.IPNet) error {
	n := newIPNet(network)

	if _, exists := allocatedIPs[n]; !exists {
		allocatedIPs[n] = &iPSet{}
	}
	if _, exists := availableIPS[n]; !exists {
		availableIPS[n] = &iPSet{}
	}

	return nil
}

// RequestIP requests an available ip from the given network.  It
// will return the next available ip if the ip provided is nil.  If the
// ip provided is not nil it will validate that the provided ip is available
// for use or return an error
func RequestIP(network *net.IPNet, ip *net.IP) (*net.IP, error) {
	lock.Lock()
	defer lock.Unlock()

	if !networkExists(network) {
		return nil, ErrNetworkNotRegistered
	}

	if ip == nil {
		next, err := getNextIp(network)
		if err != nil {
			return nil, err
		}
		return next, nil
	}

	if err := registerIP(network, ip); err != nil {
		return nil, err
	}
	return ip, nil
}

// ReleaseIP adds the provided ip back into the pool of
// available ips to be returned for use.
func ReleaseIP(network *net.IPNet, ip *net.IP) error {
	lock.Lock()
	defer lock.Unlock()

	if !networkExists(network) {
		return ErrNetworkNotRegistered
	}

	var (
		n         = newIPNet(network)
		existing  = allocatedIPs[n]
		available = availableIPS[n]
		pos       = getPosition(network, ip)
	)

	existing.Remove(int(pos))
	available.Push(int(pos))

	return nil
}

// convert the ip into the position in the subnet.  Only
// position are saved in the set
func getPosition(network *net.IPNet, ip *net.IP) int32 {
	var (
		first, _ = networkRange(network)
		base     = ipToInt(&first)
		i        = ipToInt(ip)
	)
	return i - base
}

// return an available ip if one is currently available.  If not,
// return the next available ip for the nextwork
func getNextIp(network *net.IPNet) (*net.IP, error) {
	var (
		n         = newIPNet(network)
		ownIP     = ipToInt(&network.IP)
		available = availableIPS[n]
		allocated = allocatedIPs[n]
		first, _  = networkRange(network)
		base      = ipToInt(&first)
		size      = int(networkSize(network.Mask))
		max       = int32(size - 2) // size -1 for the broadcast address, -1 for the gateway address
		pos       = int32(available.Pop())
	)

	// We pop and push the position not the ip
	if pos != 0 {
		ip := intToIP(int32(base + pos))
		allocated.Push(int(pos))

		return ip, nil
	}

	pos = int32(allocated.PullBack())
	for i := int32(0); i < max; i++ {
		pos = pos%max + 1
		next := int32(base + pos)

		if next == ownIP {
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

func registerIP(network *net.IPNet, ip *net.IP) error {
	var (
		n         = newIPNet(network)
		existing  = allocatedIPs[n]
		available = availableIPS[n]
		pos       = getPosition(network, ip)
	)

	if existing.Exists(int(pos)) {
		return ErrIPAlreadyAllocated
	}
	available.Remove(int(pos))

	return nil
}

func checkRouteOverlaps(networks []netlink.Route, toCheck *net.IPNet) error {
	for _, network := range networks {
		if network.IPNet != nil && networkOverlaps(toCheck, network.IPNet) {
			return ErrNetworkAlreadyAllocated
		}
	}
	return nil
}

// Detects overlap between one IPNet and another
func networkOverlaps(netX *net.IPNet, netY *net.IPNet) bool {
	if firstIP, _ := networkRange(netX); netY.Contains(firstIP) {
		return true
	}
	if firstIP, _ := networkRange(netY); netX.Contains(firstIP) {
		return true
	}
	return false
}

func checkExistingNetworkOverlaps(network *net.IPNet) error {
	for existing := range allocatedIPs {
		if newIPNet(network) == existing {
			return ErrNetworkAlreadyRegisterd
		}

		ex := newNetIPNet(existing)
		if networkOverlaps(network, ex) {
			return ErrNetworkAlreadyAllocated
		}
	}
	return nil
}

// Calculates the first and last IP addresses in an IPNet
func networkRange(network *net.IPNet) (net.IP, net.IP) {
	var (
		netIP   = network.IP.To4()
		firstIP = netIP.Mask(network.Mask)
		lastIP  = net.IPv4(0, 0, 0, 0).To4()
	)

	for i := 0; i < len(lastIP); i++ {
		lastIP[i] = netIP[i] | ^network.Mask[i]
	}
	return firstIP, lastIP
}

func newIPNet(network *net.IPNet) iPNet {
	return iPNet{
		IP:   string(network.IP),
		Mask: string(network.Mask),
	}
}

func newNetIPNet(network iPNet) *net.IPNet {
	return &net.IPNet{
		IP:   []byte(network.IP),
		Mask: []byte(network.Mask),
	}
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

// Given a netmask, calculates the number of available hosts
func networkSize(mask net.IPMask) int32 {
	m := net.IPv4Mask(0, 0, 0, 0)
	for i := 0; i < net.IPv4len; i++ {
		m[i] = ^mask[i]
	}

	return int32(binary.BigEndian.Uint32(m)) + 1
}

func checkNameserverOverlaps(nameservers []string, toCheck *net.IPNet) error {
	if len(nameservers) > 0 {
		for _, ns := range nameservers {
			_, nsNetwork, err := net.ParseCIDR(ns)
			if err != nil {
				return err
			}
			if networkOverlaps(toCheck, nsNetwork) {
				return ErrNetworkOverlapsWithNameservers
			}
		}
	}
	return nil
}

func networkExists(network *net.IPNet) bool {
	n := newIPNet(network)
	_, exists := allocatedIPs[n]
	return exists
}
