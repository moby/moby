package docker

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"os/exec"
	"strconv"
	"strings"
)

const (
	networkBridgeIface = "lxcbr0"
	portRangeStart     = 49153
	portRangeEnd       = 65535
)

// Calculates the first and last IP addresses in an IPNet
func networkRange(network *net.IPNet) (net.IP, net.IP) {
	netIP := network.IP.To4()
	firstIP := netIP.Mask(network.Mask)
	lastIP := net.IPv4(0, 0, 0, 0).To4()
	for i := 0; i < len(lastIP); i++ {
		lastIP[i] = netIP[i] | ^network.Mask[i]
	}
	return firstIP, lastIP
}

// Converts a 4 bytes IP into a 32 bit integer
func ipToInt(ip net.IP) (int32, error) {
	buf := bytes.NewBuffer(ip.To4())
	var n int32
	if err := binary.Read(buf, binary.BigEndian, &n); err != nil {
		return 0, err
	}
	return n, nil
}

// Converts 32 bit integer into a 4 bytes IP address
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

// Given a netmask, calculates the number of available hosts
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

// Wrapper around the iptables command
func iptables(args ...string) error {
	if err := exec.Command("/sbin/iptables", args...).Run(); err != nil {
		return fmt.Errorf("iptables failed: iptables %v", strings.Join(args, " "))
	}
	return nil
}

// Return the IPv4 address of a network interface
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
		return nil, fmt.Errorf("Interface %v has no IP addresses", name)
	case len(addrs4) > 1:
		return nil, fmt.Errorf("Interface %v has more than 1 IPv4 address", name)
	}
	return addrs4[0], nil
}

// Port mapper takes care of mapping external ports to containers by setting
// up iptables rules.
// It keeps track of all mappings and is able to unmap at will
type PortMapper struct {
	mapping map[int]net.TCPAddr
}

func (mapper *PortMapper) cleanup() error {
	// Ignore errors - This could mean the chains were never set up
	iptables("-t", "nat", "-D", "PREROUTING", "-j", "DOCKER")
	iptables("-t", "nat", "-F", "DOCKER")
	iptables("-t", "nat", "-X", "DOCKER")
	mapper.mapping = make(map[int]net.TCPAddr)
	return nil
}

func (mapper *PortMapper) setup() error {
	if err := iptables("-t", "nat", "-N", "DOCKER"); err != nil {
		return errors.New("Unable to setup port networking: Failed to create DOCKER chain")
	}
	if err := iptables("-t", "nat", "-A", "PREROUTING", "-j", "DOCKER"); err != nil {
		return errors.New("Unable to setup port networking: Failed to inject docker in PREROUTING chain")
	}
	return nil
}

func (mapper *PortMapper) iptablesForward(rule string, port int, dest net.TCPAddr) error {
	return iptables("-t", "nat", rule, "DOCKER", "-p", "tcp", "--dport", strconv.Itoa(port),
		"-j", "DNAT", "--to-destination", net.JoinHostPort(dest.IP.String(), strconv.Itoa(dest.Port)))
}

func (mapper *PortMapper) Map(port int, dest net.TCPAddr) error {
	if err := mapper.iptablesForward("-A", port, dest); err != nil {
		return err
	}
	mapper.mapping[port] = dest
	return nil
}

func (mapper *PortMapper) Unmap(port int) error {
	dest, ok := mapper.mapping[port]
	if !ok {
		return errors.New("Port is not mapped")
	}
	if err := mapper.iptablesForward("-D", port, dest); err != nil {
		return err
	}
	delete(mapper.mapping, port)
	return nil
}

func newPortMapper() (*PortMapper, error) {
	mapper := &PortMapper{}
	if err := mapper.cleanup(); err != nil {
		return nil, err
	}
	if err := mapper.setup(); err != nil {
		return nil, err
	}
	return mapper, nil
}

// Port allocator: Atomatically allocate and release networking ports
type PortAllocator struct {
	ports chan (int)
}

func (alloc *PortAllocator) populate(start, end int) {
	alloc.ports = make(chan int, end-start)
	for port := start; port < end; port++ {
		alloc.ports <- port
	}
}

func (alloc *PortAllocator) Acquire() (int, error) {
	select {
	case port := <-alloc.ports:
		return port, nil
	default:
		return -1, errors.New("No more ports available")
	}
	return -1, nil
}

func (alloc *PortAllocator) Release(port int) error {
	select {
	case alloc.ports <- port:
		return nil
	default:
		return errors.New("Too many ports have been released")
	}
	return nil
}

func newPortAllocator(start, end int) (*PortAllocator, error) {
	allocator := &PortAllocator{}
	allocator.populate(start, end)
	return allocator, nil
}

// IP allocator: Atomatically allocate and release networking ports
type IPAllocator struct {
	network *net.IPNet
	queue   chan (net.IP)
}

func (alloc *IPAllocator) populate() error {
	firstIP, _ := networkRange(alloc.network)
	size, err := networkSize(alloc.network.Mask)
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
		if ip.Equal(alloc.network.IP) {
			continue
		}
		alloc.queue <- ip
	}
	return nil
}

func (alloc *IPAllocator) Acquire() (net.IP, error) {
	select {
	case ip := <-alloc.queue:
		return ip, nil
	default:
		return net.IP{}, errors.New("No more IP addresses available")
	}
	return net.IP{}, nil
}

func (alloc *IPAllocator) Release(ip net.IP) error {
	select {
	case alloc.queue <- ip:
		return nil
	default:
		return errors.New("Too many IP addresses have been released")
	}
	return nil
}

func newIPAllocator(network *net.IPNet) (*IPAllocator, error) {
	alloc := &IPAllocator{
		network: network,
	}
	if err := alloc.populate(); err != nil {
		return nil, err
	}
	return alloc, nil
}

// Network interface represents the networking stack of a container
type NetworkInterface struct {
	IPNet   net.IPNet
	Gateway net.IP

	manager  *NetworkManager
	extPorts []int
}

// Allocate an external TCP port and map it to the interface
func (iface *NetworkInterface) AllocatePort(port int) (int, error) {
	extPort, err := iface.manager.portAllocator.Acquire()
	if err != nil {
		return -1, err
	}
	if err := iface.manager.portMapper.Map(extPort, net.TCPAddr{iface.IPNet.IP, port}); err != nil {
		iface.manager.portAllocator.Release(extPort)
		return -1, err
	}
	iface.extPorts = append(iface.extPorts, extPort)
	return extPort, nil
}

// Release: Network cleanup - release all resources
func (iface *NetworkInterface) Release() error {
	for _, port := range iface.extPorts {
		if err := iface.manager.portMapper.Unmap(port); err != nil {
			log.Printf("Unable to unmap port %v: %v", port, err)
		}
		if err := iface.manager.portAllocator.Release(port); err != nil {
			log.Printf("Unable to release port %v: %v", port, err)
		}

	}
	return iface.manager.ipAllocator.Release(iface.IPNet.IP)
}

// Network Manager manages a set of network interfaces
// Only *one* manager per host machine should be used
type NetworkManager struct {
	bridgeIface   string
	bridgeNetwork *net.IPNet

	ipAllocator   *IPAllocator
	portAllocator *PortAllocator
	portMapper    *PortMapper
}

// Allocate a network interface
func (manager *NetworkManager) Allocate() (*NetworkInterface, error) {
	ip, err := manager.ipAllocator.Acquire()
	if err != nil {
		return nil, err
	}
	iface := &NetworkInterface{
		IPNet:   net.IPNet{ip, manager.bridgeNetwork.Mask},
		Gateway: manager.bridgeNetwork.IP,
		manager: manager,
	}
	return iface, nil
}

func newNetworkManager(bridgeIface string) (*NetworkManager, error) {
	addr, err := getIfaceAddr(bridgeIface)
	if err != nil {
		return nil, err
	}
	network := addr.(*net.IPNet)

	ipAllocator, err := newIPAllocator(network)
	if err != nil {
		return nil, err
	}

	portAllocator, err := newPortAllocator(portRangeStart, portRangeEnd)
	if err != nil {
		return nil, err
	}

	portMapper, err := newPortMapper()

	manager := &NetworkManager{
		bridgeIface:   bridgeIface,
		bridgeNetwork: network,
		ipAllocator:   ipAllocator,
		portAllocator: portAllocator,
		portMapper:    portMapper,
	}
	return manager, nil
}
