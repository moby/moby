package docker

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
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
func ipToInt(ip net.IP) int32 {
	return int32(binary.BigEndian.Uint32(ip.To4()))
}

// Converts 32 bit integer into a 4 bytes IP address
func intToIp(n int32) net.IP {
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

// Port mapper takes care of mapping external ports to containers by setting
// up iptables rules.
// It keeps track of all mappings and is able to unmap at will
type PortMapper struct {
	mapping map[int]net.TCPAddr
}

func (mapper *PortMapper) cleanup() error {
	// Ignore errors - This could mean the chains were never set up
	iptables("-t", "nat", "-D", "PREROUTING", "-m", "addrtype", "--dst-type", "LOCAL", "-j", "DOCKER")
	iptables("-t", "nat", "-D", "OUTPUT", "-m", "addrtype", "--dst-type", "LOCAL", "-j", "DOCKER")
	// Also cleanup rules created by older versions, or -X might fail.
	iptables("-t", "nat", "-D", "PREROUTING", "-j", "DOCKER")
	iptables("-t", "nat", "-D", "OUTPUT", "-j", "DOCKER")
	iptables("-t", "nat", "-F", "DOCKER")
	iptables("-t", "nat", "-X", "DOCKER")
	mapper.mapping = make(map[int]net.TCPAddr)
	return nil
}

func (mapper *PortMapper) setup() error {
	if err := iptables("-t", "nat", "-N", "DOCKER"); err != nil {
		return fmt.Errorf("Failed to create DOCKER chain: %s", err)
	}
	if err := iptables("-t", "nat", "-A", "PREROUTING", "-m", "addrtype", "--dst-type", "LOCAL", "-j", "DOCKER"); err != nil {
		return fmt.Errorf("Failed to inject docker in PREROUTING chain: %s", err)
	}
	if err := iptables("-t", "nat", "-A", "OUTPUT", "-m", "addrtype", "--dst-type", "LOCAL", "-j", "DOCKER"); err != nil {
		return fmt.Errorf("Failed to inject docker in OUTPUT chain: %s", err)
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
	inUse    map[int]struct{}
	fountain chan (int)
	lock     sync.Mutex
}

func (alloc *PortAllocator) runFountain() {
	for {
		for port := portRangeStart; port < portRangeEnd; port++ {
			alloc.fountain <- port
		}
	}
}

// FIXME: Release can no longer fail, change its prototype to reflect that.
func (alloc *PortAllocator) Release(port int) error {
	alloc.lock.Lock()
	delete(alloc.inUse, port)
	alloc.lock.Unlock()
	return nil
}

func (alloc *PortAllocator) Acquire(port int) (int, error) {
	if port == 0 {
		// Allocate a port from the fountain
		for port := range alloc.fountain {
			if _, err := alloc.Acquire(port); err == nil {
				return port, nil
			}
		}
		return -1, fmt.Errorf("Port generator ended unexpectedly")
	}
	alloc.lock.Lock()
	defer alloc.lock.Unlock()
	if _, inUse := alloc.inUse[port]; inUse {
		return -1, fmt.Errorf("Port already in use: %d", port)
	}
	alloc.inUse[port] = struct{}{}
	return port, nil
}

func newPortAllocator() (*PortAllocator, error) {
	allocator := &PortAllocator{
		inUse: make(map[int]struct{}),
		fountain: make(chan int),
	}
	go allocator.runFountain()
	return allocator, nil
}

// IP allocator: Atomatically allocate and release networking ports
type IPAllocator struct {
	network       *net.IPNet
	queueAlloc    chan allocatedIP
	queueReleased chan net.IP
	inUse         map[int32]struct{}
}

type allocatedIP struct {
	ip  net.IP
	err error
}

func (alloc *IPAllocator) run() {
	firstIP, _ := networkRange(alloc.network)
	ipNum := ipToInt(firstIP)
	ownIP := ipToInt(alloc.network.IP)
	size := networkSize(alloc.network.Mask)

	pos := int32(1)
	max := size - 2 // -1 for the broadcast address, -1 for the gateway address
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

		ip := allocatedIP{ip: intToIp(newNum)}
		if inUse {
			ip.err = errors.New("No unallocated IP available")
		}

		select {
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

func (alloc *IPAllocator) Acquire() (net.IP, error) {
	ip := <-alloc.queueAlloc
	return ip.ip, ip.err
}

func (alloc *IPAllocator) Release(ip net.IP) {
	alloc.queueReleased <- ip
}

func newIPAllocator(network *net.IPNet) *IPAllocator {
	alloc := &IPAllocator{
		network:       network,
		queueAlloc:    make(chan allocatedIP),
		queueReleased: make(chan net.IP),
		inUse:         make(map[int32]struct{}),
	}

	go alloc.run()

	return alloc
}

// Network interface represents the networking stack of a container
type NetworkInterface struct {
	IPNet   net.IPNet
	Gateway net.IP

	manager  *NetworkManager
	extPorts []int
}

// Allocate an external TCP port and map it to the interface
func (iface *NetworkInterface) AllocatePort(spec string) (*Nat, error) {
	nat, err := parseNat(spec)
	if err != nil {
		return nil, err
	}
	// Allocate a random port if Frontend==0
	if extPort, err := iface.manager.portAllocator.Acquire(nat.Frontend); err != nil {
		return nil, err
	} else {
		nat.Frontend = extPort
	}
	if err := iface.manager.portMapper.Map(nat.Frontend, net.TCPAddr{IP: iface.IPNet.IP, Port: nat.Backend}); err != nil {
		iface.manager.portAllocator.Release(nat.Frontend)
		return nil, err
	}
	iface.extPorts = append(iface.extPorts, nat.Frontend)
	return nat, nil
}

type Nat struct {
	Proto    string
	Frontend int
	Backend  int
}

func parseNat(spec string) (*Nat, error) {
	var nat Nat
	// If spec starts with ':', external and internal ports must be the same.
	// This might fail if the requested external port is not available.
	var sameFrontend bool
	if spec[0] == ':' {
		sameFrontend = true
		spec = spec[1:]
	}
	port, err := strconv.ParseUint(spec, 10, 16)
	if err != nil {
		return nil, err
	}
	nat.Backend = int(port)
	if sameFrontend {
		nat.Frontend = nat.Backend
	}
	nat.Proto = "tcp"
	return &nat, nil
}

// Release: Network cleanup - release all resources
func (iface *NetworkInterface) Release() {
	for _, port := range iface.extPorts {
		if err := iface.manager.portMapper.Unmap(port); err != nil {
			log.Printf("Unable to unmap port %v: %v", port, err)
		}
		if err := iface.manager.portAllocator.Release(port); err != nil {
			log.Printf("Unable to release port %v: %v", port, err)
		}

	}

	iface.manager.ipAllocator.Release(iface.IPNet.IP)
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
		IPNet:   net.IPNet{IP: ip, Mask: manager.bridgeNetwork.Mask},
		Gateway: manager.bridgeNetwork.IP,
		manager: manager,
	}
	return iface, nil
}

func newNetworkManager(bridgeIface string) (*NetworkManager, error) {
	addr, err := getIfaceAddr(bridgeIface)
	if err != nil {
		return nil, fmt.Errorf("Couldn't find bridge interface %s (%s).\nPlease create it with 'ip link add lxcbr0 type bridge; ip addr add ADDRESS/MASK dev lxcbr0'", bridgeIface, err)
	}
	network := addr.(*net.IPNet)

	ipAllocator := newIPAllocator(network)

	portAllocator, err := newPortAllocator()
	if err != nil {
		return nil, err
	}

	portMapper, err := newPortMapper()
	if err != nil {
		return nil, err
	}

	manager := &NetworkManager{
		bridgeIface:   bridgeIface,
		bridgeNetwork: network,
		ipAllocator:   ipAllocator,
		portAllocator: portAllocator,
		portMapper:    portMapper,
	}
	return manager, nil
}
