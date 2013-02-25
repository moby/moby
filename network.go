package docker

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
)

const (
	networkBridgeIface = "lxcbr0"
)

type NetworkInterface struct {
	IPNet   net.IPNet
	Gateway net.IP
}

// IP utils

func networkRange(network *net.IPNet) (net.IP, net.IP) {
	netIP := network.IP.To4()
	firstIP := netIP.Mask(network.Mask)
	lastIP := net.IPv4(0, 0, 0, 0).To4()
	for i := 0; i < len(lastIP); i++ {
		lastIP[i] = netIP[i] | ^network.Mask[i]
	}
	return firstIP, lastIP
}

func ipToInt(ip net.IP) (int32, error) {
	buf := bytes.NewBuffer(ip.To4())
	var n int32
	if err := binary.Read(buf, binary.BigEndian, &n); err != nil {
		return 0, err
	}
	return n, nil
}

func intToIp(n int32) (net.IP, error) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, &n); err != nil {
		return net.IP{}, err
	}
	ip := net.IPv4(0, 0, 0, 0).To4()
	for i := 0; i < net.IPv4len; i++ {
		ip[i] = buf.Bytes()[i]
	}
	return ip, nil
}

func networkSize(mask net.IPMask) (int32, error) {
	m := net.IPv4Mask(0, 0, 0, 0)
	for i := 0; i < net.IPv4len; i++ {
		m[i] = ^mask[i]
	}
	buf := bytes.NewBuffer(m)
	var n int32
	if err := binary.Read(buf, binary.BigEndian, &n); err != nil {
		return 0, err
	}
	return n + 1, nil
}

func getIfaceAddr(name string) (net.Addr, error) {
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

// Network allocator
func newNetworkAllocator(iface string) (*NetworkAllocator, error) {
	addr, err := getIfaceAddr(iface)
	if err != nil {
		return nil, err
	}
	network := addr.(*net.IPNet)

	alloc := &NetworkAllocator{
		iface: iface,
		net:   network,
	}
	if err := alloc.populateFromNetwork(network); err != nil {
		return nil, err
	}
	return alloc, nil
}

type NetworkAllocator struct {
	iface string
	net   *net.IPNet
	queue chan (net.IP)
}

func (alloc *NetworkAllocator) acquireIP() (net.IP, error) {
	select {
	case ip := <-alloc.queue:
		return ip, nil
	default:
		return net.IP{}, errors.New("No more IP addresses available")
	}
	return net.IP{}, nil
}

func (alloc *NetworkAllocator) releaseIP(ip net.IP) error {
	select {
	case alloc.queue <- ip:
		return nil
	default:
		return errors.New("Too many IP addresses have been released")
	}
	return nil
}

func (alloc *NetworkAllocator) populateFromNetwork(network *net.IPNet) error {
	firstIP, _ := networkRange(network)
	size, err := networkSize(network.Mask)
	if err != nil {
		return err
	}
	// The queue size should be the network size - 3
	// -1 for the network address, -1 for the broadcast address and
	// -1 for the gateway address
	alloc.queue = make(chan net.IP, size-3)
	for i := int32(1); i < size-1; i++ {
		ipNum, err := ipToInt(firstIP)
		if err != nil {
			return err
		}
		ip, err := intToIp(ipNum + int32(i))
		if err != nil {
			return err
		}
		// Discard the network IP (that's the host IP address)
		if ip.Equal(network.IP) {
			continue
		}
		alloc.releaseIP(ip)
	}
	return nil
}

func (alloc *NetworkAllocator) Allocate() (*NetworkInterface, error) {
	// ipPrefixLen, _ := alloc.net.Mask.Size()
	ip, err := alloc.acquireIP()
	if err != nil {
		return nil, err
	}
	iface := &NetworkInterface{
		IPNet:   net.IPNet{ip, alloc.net.Mask},
		Gateway: alloc.net.IP,
	}
	return iface, nil
}

func (alloc *NetworkAllocator) Release(iface *NetworkInterface) error {
	return alloc.releaseIP(iface.IPNet.IP)
}
