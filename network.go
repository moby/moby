package docker

import (
	"bytes"
	"encoding/binary"
	"errors"
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
	for i := 0; i < net.IPv4len; i++ {
		mask[i] = ^mask[i]
	}
	buf := bytes.NewBuffer(mask)
	var n int32
	if err := binary.Read(buf, binary.BigEndian, &n); err != nil {
		return 0, err
	}
	return n + 1, nil
}

func allocateIPAddress(network *net.IPNet) (net.IP, error) {
	ip, _ := networkRange(network)
	netSize, err := networkSize(network.Mask)
	if err != nil {
		return net.IP{}, err
	}
	numIp, err := ipToInt(ip)
	if err != nil {
		return net.IP{}, err
	}
	numIp += rand.Int31n(netSize)
	return intToIp(numIp)
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
	ip, err := allocateIPAddress(bridge)
	if err != nil {
		return nil, err
	}
	iface := &NetworkInterface{
		IpAddress:   ip.String(),
		IpPrefixLen: ipPrefixLen,
		Gateway:     bridge.IP,
	}
	return iface, nil
}

type NetworkAllocator struct {
	iface string
	queue chan (net.IP)
}

func (alloc *NetworkAllocator) Acquire() (net.IP, error) {
	select {
	case ip := <-alloc.queue:
		return ip, nil
	default:
		return net.IP{}, errors.New("No more IP addresses available")
	}
	return net.IP{}, nil
}

func (alloc *NetworkAllocator) Release(ip net.IP) error {
	select {
	case alloc.queue <- ip:
		return nil
	default:
		return errors.New("Too many IP addresses have been released")
	}
	return nil
}

func (alloc *NetworkAllocator) PopulateFromNetwork(network *net.IPNet) error {
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
		alloc.Release(ip)
	}
	return nil
}

func (alloc *NetworkAllocator) PopulateFromInterface(iface string) error {
	addr, err := getBridgeAddr(iface)
	if err != nil {
		return err
	}
	network := addr.(*net.IPNet)
	return alloc.PopulateFromNetwork(network)
}
