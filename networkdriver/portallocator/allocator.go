package ipallocator

import (
	"encoding/binary"
	"errors"
	"github.com/dotcloud/docker/pkg/netlink"
	"net"
	"sort"
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
	lock                       = sync.Mutex{}
	allocatedIPs               = networkSet{}
	availableIPS               = networkSet{}
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

	allocatedIPs[newIPNet(network)] = iPSet{}

	return nil
}

func RequestIP(ip *net.IPAddr) (*net.IPAddr, error) {
	lock.Lock()
	defer lock.Unlock()

	if ip == nil {
		next, err := getNextIp()
		if err != nil {
			return nil, err
		}
		return next, nil
	}

	if err := validateIP(ip); err != nil {
		return nil, err
	}
	return ip, nil
}

func ReleaseIP(ip *net.IPAddr) error {
	lock.Lock()
	defer lock.Unlock()

}

func getNextIp(network iPNet) (net.IPAddr, error) {
	if available, exists := availableIPS[network]; exists {
	}

	var (
		netNetwork = newNetIPNet(network)
		firstIP, _ = networkRange(netNetwork)
		ipNum      = ipToInt(firstIP)
		ownIP      = ipToInt(netNetwork.IP)
		size       = networkSize(netNetwork.Mask)

		pos = int32(1)
		max = size - 2 // -1 for the broadcast address, -1 for the gateway address
	)

	for {
		var (
			newNum int32
			inUse  bool
		)

		// Find first unused IP, give up after one whole round
		for attempt := int32(0); attempt < max; attempt++ {
			newNum = ipNum + pos

			pos = pos%max + 1

			// The network's IP is never okay to use
			if newNum == ownIP {
				continue
			}

			if _, inUse = alloc.inUse[newNum]; !inUse {
				// We found an unused IP
				break
			}
		}

		ip := allocatedIP{ip: intToIP(newNum)}
		if inUse {
			ip.err = errors.New("No unallocated IP available")
		}

		select {
		case quit := <-alloc.quit:
			if quit {
				return
			}
		case alloc.queueAlloc <- ip:
			alloc.inUse[newNum] = struct{}{}
		case released := <-alloc.queueReleased:
			r := ipToInt(released)
			delete(alloc.inUse, r)

			if inUse {
				// If we couldn't allocate a new IP, the released one
				// will be the only free one now, so instantly use it
				// next time
				pos = r - ipNum
			} else {
				// Use same IP as last time
				if pos == 1 {
					pos = max
				} else {
					pos--
				}
			}
		}
	}

}

func validateIP(ip *net.IPAddr) error {

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
		if networkOverlaps(network, existing) {
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
func ipToInt(ip net.IP) int32 {
	return int32(binary.BigEndian.Uint32(ip.To4()))
}

// Converts 32 bit integer into a 4 bytes IP address
func intToIP(n int32) net.IP {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(n))
	return net.IP(b)
}

// Given a netmask, calculates the number of available hosts
func networkSize(mask net.IPMask) int32 {
	m := net.IPv4Mask(0, 0, 0, 0)
	for i := 0; i < net.IPv4len; i++ {
		m[i] = ^mask[i]
	}

	return int32(binary.BigEndian.Uint32(m)) + 1
}
