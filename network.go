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
	path, err := exec.LookPath("iptables")
	if err != nil {
		return fmt.Errorf("command not found: iptables")
	}
	if err := exec.Command(path, args...).Run(); err != nil {
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
		fmt.Printf("Interface %v has more than 1 IPv4 address. Defaulting to using %v\n",
			name, (addrs4[0].(*net.IPNet)).IP)
	}
	return addrs4[0], nil
}

// addr is a compact generic representation of an IP and port (of any
// protocol), adequate to be stored in memory.
type addr struct {
	ip   net.IP
	port int
}

// A Port holds represents a network port of a given protocol (e.g: "udp",
// "tcp"). Each Port repeats the name of the protocol, so this is intended for
// serialization in configuration files only and not for being stored
// repeatedly in memory.
type Port struct {
	Proto string
	Num   int
}

func (p Port) String() string {
	return fmt.Sprintf("%v/%d", p.Proto, p.Num)
}

func newPort(p string) (Port, error) {
	port, proto, err := parsePort(p)
	return Port{proto, port}, err
}

func parsePort(p string) (port int, proto string, err error) {
	s := strings.Split(p, "/")
	switch len(s) {
	case 0:
		return 0, "", fmt.Errorf("Invalid port port (empty)")
	case 1:
		// No proto provided, defaults to TCP.
		proto = "tcp"
		port, err = strconv.Atoi(s[0])
	case 2:
		proto = s[0]
		port, err = strconv.Atoi(s[1])
	default:
		err = fmt.Errorf("Invalid port format: %v", port)
	}
	if err != nil {
		return 0, "", err
	}
	return
}

// Port mapper takes care of mapping external ports to containers by setting up
// iptables rules. There is one PortMapper for each transport protocol (tcp,
// udp). It keeps track of all mappings and is able to unmap at will
type PortMapper struct {
	proto   string
	mapping map[int]addr
}

func (mapper *PortMapper) cleanup() error {
	// Ignore errors - This could mean the chains were never set up
	iptables("-t", "nat", "-D", "PREROUTING", "-j", "DOCKER")
	iptables("-t", "nat", "-D", "OUTPUT", "-j", "DOCKER")
	iptables("-t", "nat", "-F", "DOCKER")
	iptables("-t", "nat", "-X", "DOCKER")
	mapper.mapping = make(map[int]addr)
	return nil
}

func (mapper *PortMapper) setup() error {
	if err := iptables("-t", "nat", "-N", "DOCKER"); err != nil {
		return fmt.Errorf("Failed to create DOCKER chain: %s", err)
	}
	if err := iptables("-t", "nat", "-A", "PREROUTING", "-j", "DOCKER"); err != nil {
		return fmt.Errorf("Failed to inject docker in PREROUTING chain: %s", err)
	}
	if err := iptables("-t", "nat", "-A", "OUTPUT", "-j", "DOCKER"); err != nil {
		return fmt.Errorf("Failed to inject docker in OUTPUT chain: %s", err)
	}
	return nil
}

func (mapper *PortMapper) iptablesForward(rule string, port int, dest addr) error {
	return iptables("-t", "nat", rule, "DOCKER", "-p", mapper.proto, "--dport", strconv.Itoa(port),
		"-j", "DNAT", "--to-destination", net.JoinHostPort(dest.ip.String(), strconv.Itoa(dest.port)))
}

func (mapper *PortMapper) Map(port int, dest addr) error {
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

func newPortMapper(proto string) (*PortMapper, error) {
	mapper := &PortMapper{proto: proto}
	if err := mapper.cleanup(); err != nil {
		return nil, err
	}
	if err := mapper.setup(); err != nil {
		return nil, err
	}
	return mapper, nil
}

// Port allocator: Atomatically allocate and release networking ports for a
// specific protocol. Separate PortAllocators are needed for TCP, UDP, etc.
type PortAllocator struct {
	ports chan (int)
}

func (alloc *PortAllocator) populate(start, end int) {
	alloc.ports = make(chan int, end-start)
	for port := start; port < end; port++ {
		alloc.ports <- port
	}
}

func (alloc *PortAllocator) Acquire() (port int, err error) {
	select {
	case port = <-alloc.ports:
		return port, nil
	default:
		return port, errors.New("No more ports available")
	}
	return port, nil
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
	extPorts []Port
}

// Allocate an external port and map it to the interface
func (iface *NetworkInterface) AllocatePort(proto string, port int) (int, error) {
	portManager, err := iface.portManager(proto)
	if err != nil {
		return 0, err
	}
	extPort, err := portManager.Acquire()
	if err != nil {
		return 0, err
	}
	if err := portManager.Map(extPort, addr{ip: iface.IPNet.IP, port: port}); err != nil {
		portManager.Release(extPort)
		return 0, err
	}
	iface.extPorts = append(iface.extPorts, Port{proto, extPort})
	return extPort, nil
}

func (iface *NetworkInterface) portManager(proto string) (*PortManager, error) {
	switch proto {
	case "tcp":
		return iface.manager.tcpManager, nil
	case "udp":
		return iface.manager.udpManager, nil
	}
	return nil, fmt.Errorf("Can't find port manager for unknown protocol %v", proto)
}

// Release: Network cleanup - release all resources
func (iface *NetworkInterface) Release() error {
	for _, port := range iface.extPorts {
		portManager, err := iface.portManager(port.Proto)
		if err != nil {
			return err
		}
		if err := portManager.Unmap(port.Num); err != nil {
			log.Printf("Unable to unmap port %v: %v", port, err)
		}
		if err := portManager.Release(port.Num); err != nil {
			log.Printf("Unable to release port %v: %v", port, err)
		}
	}
	return iface.manager.ipAllocator.Release(iface.IPNet.IP)
}

func newPortManager(proto string) (*PortManager, error) {
	portAllocator, err := newPortAllocator(
		portRangeStart,
		portRangeEnd,
	)
	if err != nil {
		return nil, err
	}
	portMapper, err := newPortMapper(proto)
	if err != nil {
		return nil, err
	}
	return &PortManager{portAllocator, portMapper, proto}, nil
}

// PortManager is a wrapper for PortAllocator and PortMapper, for controlling
// allocations for a particular protocol (e.g: tcp, udp).
type PortManager struct {
	*PortAllocator
	*PortMapper
	proto string
}

// Network Manager manages a set of network interfaces
// Only *one* manager per host machine should be used
type NetworkManager struct {
	bridgeIface   string
	bridgeNetwork *net.IPNet

	ipAllocator *IPAllocator
	tcpManager  *PortManager
	udpManager  *PortManager
}

// Allocate a network interface
func (manager *NetworkManager) Allocate() (*NetworkInterface, error) {
	ip, err := manager.ipAllocator.Acquire()
	if err != nil {
		return nil, err
	}
	iface := &NetworkInterface{
		IPNet:   net.IPNet{IP: ip, Mask: manager.bridgeNetwork.Mask},
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
	tcpManager, err := newPortManager("tcp")
	if err != nil {
		return nil, err
	}
	udpManager, err := newPortManager("udp")
	if err != nil {
		return nil, err
	}
	manager := &NetworkManager{
		bridgeIface:   bridgeIface,
		bridgeNetwork: network,
		ipAllocator:   ipAllocator,
		tcpManager:    tcpManager,
		udpManager:    udpManager,
	}
	return manager, nil
}
