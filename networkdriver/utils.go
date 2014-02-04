package networkdriver

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"

	"github.com/dotcloud/docker/pkg/netlink"
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
	if firstIP, _ := NetworkRange(netX); netY.Contains(firstIP) {
		return true
	}
	if firstIP, _ := NetworkRange(netY); netX.Contains(firstIP) {
		return true
	}
	return false
}

// Calculates the first and last IP addresses in an IPNet
func NetworkRange(network *net.IPNet) (net.IP, net.IP) {
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

// Given a netmask, calculates the number of available hosts
func NetworkSize(mask net.IPMask) int32 {
	m := net.IPv4Mask(0, 0, 0, 0)
	for i := 0; i < net.IPv4len; i++ {
		m[i] = ^mask[i]
	}
	return int32(binary.BigEndian.Uint32(m)) + 1
}

// Return the IPv4 address of a network interface
func GetIfaceAddr(name string) (net.Addr, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	var addrs4 []net.Addr
	for _, addr := range addrs {
		ip := (addr.(*net.IPNet)).IP
		if ip4 := ip.To4(); len(ip4) == net.IPv4len {
			addrs4 = append(addrs4, addr)
		}
	}
	switch {
	case len(addrs4) == 0:
		return nil, fmt.Errorf("Interface %v has no IP addresses", name)
	case len(addrs4) > 1:
		fmt.Printf("Interface %v has more than 1 IPv4 address. Defaulting to using %v\n",
			name, (addrs4[0].(*net.IPNet)).IP)
	}
	return addrs4[0], nil
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
