package docker

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/dotcloud/docker/utils"
	"log"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

var NetworkBridgeIface string

const (
	DefaultNetworkBridge = "docker0"
	DisableNetworkBridge = "none"
	portRangeStart       = 49153
	portRangeEnd         = 65535
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

// Detects overlap between one IPNet and another
func networkOverlaps(netX *net.IPNet, netY *net.IPNet) bool {
	firstIP, _ := networkRange(netX)
	if netY.Contains(firstIP) {
		return true
	}
	firstIP, _ = networkRange(netY)
	if netX.Contains(firstIP) {
		return true
	}
	return false
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

//Wrapper around the ip command
func ip(args ...string) (string, error) {
	path, err := exec.LookPath("ip")
	if err != nil {
		return "", fmt.Errorf("command not found: ip")
	}
	output, err := exec.Command(path, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ip failed: ip %v", strings.Join(args, " "))
	}
	return string(output), nil
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

func checkRouteOverlaps(dockerNetwork *net.IPNet) error {
	output, err := ip("route")
	if err != nil {
		return err
	}
	utils.Debugf("Routes:\n\n%s", output)
	for _, line := range strings.Split(output, "\n") {
		if strings.Trim(line, "\r\n\t ") == "" || strings.Contains(line, "default") {
			continue
		}
		if _, network, err := net.ParseCIDR(strings.Split(line, " ")[0]); err != nil {
			return fmt.Errorf("Unexpected ip route output: %s (%s)", err, line)
		} else if networkOverlaps(dockerNetwork, network) {
			return fmt.Errorf("Network %s is already routed: '%s'", dockerNetwork.String(), line)
		}
	}
	return nil
}

// CreateBridgeIface creates a network bridge interface on the host system with the name `ifaceName`,
// and attempts to configure it with an address which doesn't conflict with any other interface on the host.
// If it can't find an address which doesn't conflict, it will return an error.
func CreateBridgeIface(ifaceName string) error {
	addrs := []string{
		// Here we don't follow the convention of using the 1st IP of the range for the gateway.
		// This is to use the same gateway IPs as the /24 ranges, which predate the /16 ranges.
		// In theory this shouldn't matter - in practice there's bound to be a few scripts relying
		// on the internal addressing or other stupid things like that.
		// The shouldn't, but hey, let's not break them unless we really have to.
		"172.17.42.1/16", // Don't use 172.16.0.0/16, it conflicts with EC2 DNS 172.16.0.23
		"10.0.42.1/16",   // Don't even try using the entire /8, that's too intrusive
		"10.1.42.1/16",
		"10.42.42.1/16",
		"172.16.42.1/24",
		"172.16.43.1/24",
		"172.16.44.1/24",
		"10.0.42.1/24",
		"10.0.43.1/24",
		"192.168.42.1/24",
		"192.168.43.1/24",
		"192.168.44.1/24",
	}

	var ifaceAddr string
	for _, addr := range addrs {
		_, dockerNetwork, err := net.ParseCIDR(addr)
		if err != nil {
			return err
		}
		if err := checkRouteOverlaps(dockerNetwork); err == nil {
			ifaceAddr = addr
			break
		} else {
			utils.Debugf("%s: %s", addr, err)
		}
	}
	if ifaceAddr == "" {
		return fmt.Errorf("Could not find a free IP address range for interface '%s'. Please configure its address manually and run 'docker -b %s'", ifaceName, ifaceName)
	}
	utils.Debugf("Creating bridge %s with network %s", ifaceName, ifaceAddr)

	if output, err := ip("link", "add", ifaceName, "type", "bridge"); err != nil {
		return fmt.Errorf("Error creating bridge: %s (output: %s)", err, output)
	}

	if output, err := ip("addr", "add", ifaceAddr, "dev", ifaceName); err != nil {
		return fmt.Errorf("Unable to add private network: %s (%s)", err, output)
	}
	if output, err := ip("link", "set", ifaceName, "up"); err != nil {
		return fmt.Errorf("Unable to start network bridge: %s (%s)", err, output)
	}
	if err := iptables("-t", "nat", "-A", "POSTROUTING", "-s", ifaceAddr,
		"!", "-d", ifaceAddr, "-j", "MASQUERADE"); err != nil {
		return fmt.Errorf("Unable to enable network bridge NAT: %s", err)
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
	tcpMapping map[int]*net.TCPAddr
	tcpProxies map[int]Proxy
	udpMapping map[int]*net.UDPAddr
	udpProxies map[int]Proxy
}

func (mapper *PortMapper) cleanup() error {
	// Ignore errors - This could mean the chains were never set up
	iptables("-t", "nat", "-D", "PREROUTING", "-m", "addrtype", "--dst-type", "LOCAL", "-j", "DOCKER")
	iptables("-t", "nat", "-D", "OUTPUT", "-m", "addrtype", "--dst-type", "LOCAL", "!", "--dst", "127.0.0.0/8", "-j", "DOCKER")
	iptables("-t", "nat", "-D", "OUTPUT", "-m", "addrtype", "--dst-type", "LOCAL", "-j", "DOCKER") // Created in versions <= 0.1.6
	// Also cleanup rules created by older versions, or -X might fail.
	iptables("-t", "nat", "-D", "PREROUTING", "-j", "DOCKER")
	iptables("-t", "nat", "-D", "OUTPUT", "-j", "DOCKER")
	iptables("-t", "nat", "-F", "DOCKER")
	iptables("-t", "nat", "-X", "DOCKER")
	mapper.tcpMapping = make(map[int]*net.TCPAddr)
	mapper.tcpProxies = make(map[int]Proxy)
	mapper.udpMapping = make(map[int]*net.UDPAddr)
	mapper.udpProxies = make(map[int]Proxy)
	return nil
}

func (mapper *PortMapper) setup() error {
	if err := iptables("-t", "nat", "-N", "DOCKER"); err != nil {
		return fmt.Errorf("Failed to create DOCKER chain: %s", err)
	}
	if err := iptables("-t", "nat", "-A", "PREROUTING", "-m", "addrtype", "--dst-type", "LOCAL", "-j", "DOCKER"); err != nil {
		return fmt.Errorf("Failed to inject docker in PREROUTING chain: %s", err)
	}
	if err := iptables("-t", "nat", "-A", "OUTPUT", "-m", "addrtype", "--dst-type", "LOCAL", "!", "--dst", "127.0.0.0/8", "-j", "DOCKER"); err != nil {
		return fmt.Errorf("Failed to inject docker in OUTPUT chain: %s", err)
	}
	return nil
}

func (mapper *PortMapper) iptablesForward(rule string, port int, proto string, dest_addr string, dest_port int) error {
	return iptables("-t", "nat", rule, "DOCKER", "-p", proto, "--dport", strconv.Itoa(port),
		"-j", "DNAT", "--to-destination", net.JoinHostPort(dest_addr, strconv.Itoa(dest_port)))
}

func (mapper *PortMapper) Map(port int, backendAddr net.Addr) error {
	if _, isTCP := backendAddr.(*net.TCPAddr); isTCP {
		backendPort := backendAddr.(*net.TCPAddr).Port
		backendIP := backendAddr.(*net.TCPAddr).IP
		if err := mapper.iptablesForward("-A", port, "tcp", backendIP.String(), backendPort); err != nil {
			return err
		}
		mapper.tcpMapping[port] = backendAddr.(*net.TCPAddr)
		proxy, err := NewProxy(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port}, backendAddr)
		if err != nil {
			mapper.Unmap(port, "tcp")
			return err
		}
		mapper.tcpProxies[port] = proxy
		go proxy.Run()
	} else {
		backendPort := backendAddr.(*net.UDPAddr).Port
		backendIP := backendAddr.(*net.UDPAddr).IP
		if err := mapper.iptablesForward("-A", port, "udp", backendIP.String(), backendPort); err != nil {
			return err
		}
		mapper.udpMapping[port] = backendAddr.(*net.UDPAddr)
		proxy, err := NewProxy(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port}, backendAddr)
		if err != nil {
			mapper.Unmap(port, "udp")
			return err
		}
		mapper.udpProxies[port] = proxy
		go proxy.Run()
	}
	return nil
}

func (mapper *PortMapper) Unmap(port int, proto string) error {
	if proto == "tcp" {
		backendAddr, ok := mapper.tcpMapping[port]
		if !ok {
			return fmt.Errorf("Port tcp/%v is not mapped", port)
		}
		if proxy, exists := mapper.tcpProxies[port]; exists {
			proxy.Close()
			delete(mapper.tcpProxies, port)
		}
		if err := mapper.iptablesForward("-D", port, proto, backendAddr.IP.String(), backendAddr.Port); err != nil {
			return err
		}
		delete(mapper.tcpMapping, port)
	} else {
		backendAddr, ok := mapper.udpMapping[port]
		if !ok {
			return fmt.Errorf("Port udp/%v is not mapped", port)
		}
		if proxy, exists := mapper.udpProxies[port]; exists {
			proxy.Close()
			delete(mapper.udpProxies, port)
		}
		if err := mapper.iptablesForward("-D", port, proto, backendAddr.IP.String(), backendAddr.Port); err != nil {
			return err
		}
		delete(mapper.udpMapping, port)
	}
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
	sync.Mutex
	inUse    map[int]struct{}
	fountain chan (int)
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
	utils.Debugf("Releasing %d", port)
	alloc.Lock()
	delete(alloc.inUse, port)
	alloc.Unlock()
	return nil
}

func (alloc *PortAllocator) Acquire(port int) (int, error) {
	utils.Debugf("Acquiring %d", port)
	if port == 0 {
		// Allocate a port from the fountain
		for port := range alloc.fountain {
			if _, err := alloc.Acquire(port); err == nil {
				return port, nil
			}
		}
		return -1, fmt.Errorf("Port generator ended unexpectedly")
	}
	alloc.Lock()
	defer alloc.Unlock()
	if _, inUse := alloc.inUse[port]; inUse {
		return -1, fmt.Errorf("Port already in use: %d", port)
	}
	alloc.inUse[port] = struct{}{}
	return port, nil
}

func newPortAllocator() (*PortAllocator, error) {
	allocator := &PortAllocator{
		inUse:    make(map[int]struct{}),
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

		ip := allocatedIP{ip: intToIP(newNum)}
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
	extPorts []*Nat
	disabled bool
}

// Allocate an external TCP port and map it to the interface
func (iface *NetworkInterface) AllocatePort(spec string) (*Nat, error) {

	if iface.disabled {
		return nil, fmt.Errorf("Trying to allocate port for interface %v, which is disabled", iface) // FIXME
	}

	nat, err := parseNat(spec)
	if err != nil {
		return nil, err
	}

	if nat.Proto == "tcp" {
		extPort, err := iface.manager.tcpPortAllocator.Acquire(nat.Frontend)
		if err != nil {
			return nil, err
		}
		backend := &net.TCPAddr{IP: iface.IPNet.IP, Port: nat.Backend}
		if err := iface.manager.portMapper.Map(extPort, backend); err != nil {
			iface.manager.tcpPortAllocator.Release(extPort)
			return nil, err
		}
		nat.Frontend = extPort
	} else {
		extPort, err := iface.manager.udpPortAllocator.Acquire(nat.Frontend)
		if err != nil {
			return nil, err
		}
		backend := &net.UDPAddr{IP: iface.IPNet.IP, Port: nat.Backend}
		if err := iface.manager.portMapper.Map(extPort, backend); err != nil {
			iface.manager.udpPortAllocator.Release(extPort)
			return nil, err
		}
		nat.Frontend = extPort
	}
	iface.extPorts = append(iface.extPorts, nat)

	return nat, nil
}

type Nat struct {
	Proto    string
	Frontend int
	Backend  int
}

func parseNat(spec string) (*Nat, error) {
	var nat Nat

	if strings.Contains(spec, "/") {
		specParts := strings.Split(spec, "/")
		if len(specParts) != 2 {
			return nil, fmt.Errorf("Invalid port format.")
		}
		proto := specParts[1]
		spec = specParts[0]
		if proto != "tcp" && proto != "udp" {
			return nil, fmt.Errorf("Invalid port format: unknown protocol %v.", proto)
		}
		nat.Proto = proto
	} else {
		nat.Proto = "tcp"
	}

	if strings.Contains(spec, ":") {
		specParts := strings.Split(spec, ":")
		if len(specParts) != 2 {
			return nil, fmt.Errorf("Invalid port format.")
		}
		// If spec starts with ':', external and internal ports must be the same.
		// This might fail if the requested external port is not available.
		var sameFrontend bool
		if len(specParts[0]) == 0 {
			sameFrontend = true
		} else {
			front, err := strconv.ParseUint(specParts[0], 10, 16)
			if err != nil {
				return nil, err
			}
			nat.Frontend = int(front)
		}
		back, err := strconv.ParseUint(specParts[1], 10, 16)
		if err != nil {
			return nil, err
		}
		nat.Backend = int(back)
		if sameFrontend {
			nat.Frontend = nat.Backend
		}
	} else {
		port, err := strconv.ParseUint(spec, 10, 16)
		if err != nil {
			return nil, err
		}
		nat.Backend = int(port)
	}

	return &nat, nil
}

// Release: Network cleanup - release all resources
func (iface *NetworkInterface) Release() {

	if iface.disabled {
		return
	}

	for _, nat := range iface.extPorts {
		utils.Debugf("Unmaping %v/%v", nat.Proto, nat.Frontend)
		if err := iface.manager.portMapper.Unmap(nat.Frontend, nat.Proto); err != nil {
			log.Printf("Unable to unmap port %v/%v: %v", nat.Proto, nat.Frontend, err)
		}
		if nat.Proto == "tcp" {
			if err := iface.manager.tcpPortAllocator.Release(nat.Frontend); err != nil {
				log.Printf("Unable to release port tcp/%v: %v", nat.Frontend, err)
			}
		} else if err := iface.manager.udpPortAllocator.Release(nat.Frontend); err != nil {
			log.Printf("Unable to release port udp/%v: %v", nat.Frontend, err)
		}
	}

	iface.manager.ipAllocator.Release(iface.IPNet.IP)
}

// Network Manager manages a set of network interfaces
// Only *one* manager per host machine should be used
type NetworkManager struct {
	bridgeIface   string
	bridgeNetwork *net.IPNet

	ipAllocator      *IPAllocator
	tcpPortAllocator *PortAllocator
	udpPortAllocator *PortAllocator
	portMapper       *PortMapper

	disabled bool
}

// Allocate a network interface
func (manager *NetworkManager) Allocate() (*NetworkInterface, error) {

	if manager.disabled {
		return &NetworkInterface{disabled: true}, nil
	}

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

	if bridgeIface == DisableNetworkBridge {
		manager := &NetworkManager{
			disabled: true,
		}
		return manager, nil
	}

	addr, err := getIfaceAddr(bridgeIface)
	if err != nil {
		// If the iface is not found, try to create it
		if err := CreateBridgeIface(bridgeIface); err != nil {
			return nil, err
		}
		addr, err = getIfaceAddr(bridgeIface)
		if err != nil {
			return nil, err
		}
	}
	network := addr.(*net.IPNet)

	ipAllocator := newIPAllocator(network)

	tcpPortAllocator, err := newPortAllocator()
	if err != nil {
		return nil, err
	}
	udpPortAllocator, err := newPortAllocator()
	if err != nil {
		return nil, err
	}

	portMapper, err := newPortMapper()
	if err != nil {
		return nil, err
	}

	manager := &NetworkManager{
		bridgeIface:      bridgeIface,
		bridgeNetwork:    network,
		ipAllocator:      ipAllocator,
		tcpPortAllocator: tcpPortAllocator,
		udpPortAllocator: udpPortAllocator,
		portMapper:       portMapper,
	}
	return manager, nil
}
