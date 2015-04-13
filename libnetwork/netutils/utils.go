// Network utility functions.
// Imported unchanged from Docker

package netutils

import (
	"errors"
	"fmt"
	"math/rand"
	"net"

	"github.com/vishvananda/netlink"
)

var (
	// ErrNetworkOverlapsWithNameservers preformatted error
	ErrNetworkOverlapsWithNameservers = errors.New("requested network overlaps with nameserver")
	// ErrNetworkOverlaps preformatted error
	ErrNetworkOverlaps = errors.New("requested network overlaps with existing network")
	// ErrNoDefaultRoute preformatted error
	ErrNoDefaultRoute = errors.New("no default route")

	networkGetRoutesFct = netlink.RouteList
)

// CheckNameserverOverlaps checks whether the passed network overlaps with any of the nameservers
func CheckNameserverOverlaps(nameservers []string, toCheck *net.IPNet) error {
	if len(nameservers) > 0 {
		for _, ns := range nameservers {
			_, nsNetwork, err := net.ParseCIDR(ns)
			if err != nil {
				return err
			}
			if NetworkOverlaps(toCheck, nsNetwork) {
				return ErrNetworkOverlapsWithNameservers
			}
		}
	}
	return nil
}

// CheckRouteOverlaps checks whether the passed network overlaps with any existing routes
func CheckRouteOverlaps(toCheck *net.IPNet) error {
	networks, err := networkGetRoutesFct(nil, netlink.FAMILY_V4)
	if err != nil {
		return err
	}

	for _, network := range networks {
		if network.Dst != nil && NetworkOverlaps(toCheck, network.Dst) {
			return ErrNetworkOverlaps
		}
	}
	return nil
}

// NetworkOverlaps detects overlap between one IPNet and another
func NetworkOverlaps(netX *net.IPNet, netY *net.IPNet) bool {
	if len(netX.IP) == len(netY.IP) {
		if firstIP, _ := NetworkRange(netX); netY.Contains(firstIP) {
			return true
		}
		if firstIP, _ := NetworkRange(netY); netX.Contains(firstIP) {
			return true
		}
	}
	return false
}

// NetworkRange calculates the first and last IP addresses in an IPNet
func NetworkRange(network *net.IPNet) (net.IP, net.IP) {
	var netIP net.IP
	if network.IP.To4() != nil {
		netIP = network.IP.To4()
	} else if network.IP.To16() != nil {
		netIP = network.IP.To16()
	} else {
		return nil, nil
	}

	lastIP := make([]byte, len(netIP), len(netIP))
	for i := 0; i < len(netIP); i++ {
		lastIP[i] = netIP[i] | ^network.Mask[i]
	}
	return netIP.Mask(network.Mask), net.IP(lastIP)
}

// GetIfaceAddr returns the first IPv4 address and slice of IPv6 addresses for the specified network interface
func GetIfaceAddr(name string) (net.Addr, []net.Addr, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, nil, err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, nil, err
	}
	var addrs4 []net.Addr
	var addrs6 []net.Addr
	for _, addr := range addrs {
		ip := (addr.(*net.IPNet)).IP
		if ip4 := ip.To4(); ip4 != nil {
			addrs4 = append(addrs4, addr)
		} else if ip6 := ip.To16(); len(ip6) == net.IPv6len {
			addrs6 = append(addrs6, addr)
		}
	}
	switch {
	case len(addrs4) == 0:
		return nil, nil, fmt.Errorf("Interface %v has no IPv4 addresses", name)
	case len(addrs4) > 1:
		fmt.Printf("Interface %v has more than 1 IPv4 address. Defaulting to using %v\n",
			name, (addrs4[0].(*net.IPNet)).IP)
	}
	return addrs4[0], addrs6, nil
}

// GenerateRandomMAC returns a random MAC address
func GenerateRandomMAC() net.HardwareAddr {
	hw := make(net.HardwareAddr, 6)
	for i := 0; i < 6; i++ {
		hw[i] = byte(rand.Intn(255))
	}
	hw[0] &^= 0x1 // clear multicast bit
	hw[0] |= 0x2  // set local assignment bit (IEEE802)
	return hw
}
