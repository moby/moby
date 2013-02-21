package docker

import (
	"fmt"
	"math/rand"
	"net"
)

const (
	networkBridgeIface = "lxcbr0"
)

type NetworkInterface struct {
	IpAddress   string
	IpPrefixLen int
	Gateway     net.IP
}

func allocateIPAddress(network *net.IPNet) net.IP {
	ip := network.IP.Mask(network.Mask)
	ip[3] = byte(rand.Intn(254))
	return ip
}

func getBridgeAddr(name string) (net.Addr, error) {
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
		return nil, fmt.Errorf("Bridge %v has no IP addresses", name)
	case len(addrs4) > 1:
		return nil, fmt.Errorf("Bridge %v has more than 1 IPv4 address", name)
	}
	return addrs4[0], nil
}

func allocateNetwork() (*NetworkInterface, error) {
	bridgeAddr, err := getBridgeAddr(networkBridgeIface)
	if err != nil {
		return nil, err
	}
	bridge := bridgeAddr.(*net.IPNet)
	ipPrefixLen, _ := bridge.Mask.Size()
	iface := &NetworkInterface{
		IpAddress:   allocateIPAddress(bridge).String(),
		IpPrefixLen: ipPrefixLen,
		Gateway:     bridge.IP,
	}
	return iface, nil
}
