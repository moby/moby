package networkdriver

import (
	"errors"
	"fmt"
	"net"

	"github.com/docker/libcontainer/netlink"
)

var (
	networkGetRoutesFct = netlink.NetworkGetRoutes
	ErrNoDefaultRoute   = errors.New("no default route")
)

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

func CheckRouteOverlaps(toCheck *net.IPNet) error {
	networks, err := networkGetRoutesFct()
	if err != nil {
		return err
	}

	for _, network := range networks {
		if network.IPNet != nil && NetworkOverlaps(toCheck, network.IPNet) {
			return ErrNetworkOverlaps
		}
	}
	return nil
}

// Detects overlap between one IPNet and another
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

// Calculates the first and last IP addresses in an IPNet
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

// Return the first IPv4 address and slice of IPv6 addresses for the specified network interface
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

func GetDefaultRouteIface() (*net.Interface, error) {
	rs, err := networkGetRoutesFct()
	if err != nil {
		return nil, fmt.Errorf("unable to get routes: %v", err)
	}
	for _, r := range rs {
		if r.Default {
			return r.Iface, nil
		}
	}
	return nil, ErrNoDefaultRoute
}
