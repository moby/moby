package ipallocator

import (
	"encoding/binary"
	"errors"
	"github.com/dotcloud/docker/pkg/netlink"
	"net"
	"sync"
)

type networkSet map[iPNet]iPSet

type iPNet struct {
	IP   string
	Mask string
}

var (
	ErrNetworkAlreadyAllocated = errors.New("requested network overlaps with existing network")
	ErrNetworkAlreadyRegisterd = errors.New("requested network is already registered")
	ErrNoAvailableIps          = errors.New("no available ips on network")
	ErrIPAlreadyAllocated      = errors.New("ip already allocated")

	lock         = sync.Mutex{}
	allocatedIPs = networkSet{}
	availableIPS = networkSet{}
)

func RegisterNetwork(network *net.IPNet) error {
	lock.Lock()
	defer lock.Unlock()

	routes, err := netlink.NetworkGetRoutes()
	if err != nil {
		return err
	}

	if err := checkRouteOverlaps(routes, network); err != nil {
		return err
	}

	if err := checkExistingNetworkOverlaps(network); err != nil {
		return err
	}
	n := newIPNet(network)

	allocatedIPs[n] = iPSet{}
	availableIPS[n] = iPSet{}

	return nil
}

func RequestIP(network *net.IPNet, ip *net.IP) (*net.IP, error) {
	lock.Lock()
	defer lock.Unlock()

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

func ReleaseIP(network *net.IPNet, ip *net.IP) error {
	lock.Lock()
	defer lock.Unlock()

	n := newIPNet(network)
	existing := allocatedIPs[n]

	i := ipToInt(ip)
	existing.Remove(int(i))
	available := availableIPS[n]
	available.Push(int(i))

	return nil
}

func getNextIp(network *net.IPNet) (*net.IP, error) {
	var (
		n         = newIPNet(network)
		available = availableIPS[n]
		next      = available.Pop()
		allocated = allocatedIPs[n]
		ownIP     = int(ipToInt(&network.IP))
	)

	if next != 0 {
		ip := intToIP(int32(next))
		allocated.Push(int(next))
		return ip, nil
	}
	size := int(networkSize(network.Mask))
	next = allocated.PullBack() + 1

	// size -1 for the broadcast address, -1 for the gateway address
	for i := 0; i < size-2; i++ {
		if next == ownIP {
			next++
			continue
		}

		ip := intToIP(int32(next))
		allocated.Push(next)

		return ip, nil
	}
	return nil, ErrNoAvailableIps
}

func registerIP(network *net.IPNet, ip *net.IP) error {
	existing := allocatedIPs[newIPNet(network)]
	if existing.Exists(int(ipToInt(ip))) {
		return ErrIPAlreadyAllocated
	}
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
