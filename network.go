package docker

import (
	"fmt"
	"github.com/dotcloud/docker/networkdriver"
	"github.com/dotcloud/docker/networkdriver/ipallocator"
	"github.com/dotcloud/docker/pkg/iptables"
	"github.com/dotcloud/docker/pkg/netlink"
	"github.com/dotcloud/docker/proxy"
	"github.com/dotcloud/docker/utils"
	"log"
	"net"
	"strconv"
	"sync"
	"syscall"
	"unsafe"
)

const (
	DefaultNetworkBridge = "docker0"
	DisableNetworkBridge = "none"
	DefaultNetworkMtu    = 1500
	portRangeStart       = 49153
	portRangeEnd         = 65535
	siocBRADDBR          = 0x89a0
)

// CreateBridgeIface creates a network bridge interface on the host system with the name `ifaceName`,
// and attempts to configure it with an address which doesn't conflict with any other interface on the host.
// If it can't find an address which doesn't conflict, it will return an error.
func CreateBridgeIface(config *DaemonConfig) error {
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

	nameservers := []string{}
	resolvConf, _ := utils.GetResolvConf()
	// we don't check for an error here, because we don't really care
	// if we can't read /etc/resolv.conf. So instead we skip the append
	// if resolvConf is nil. It either doesn't exist, or we can't read it
	// for some reason.
	if resolvConf != nil {
		nameservers = append(nameservers, utils.GetNameserversAsCIDR(resolvConf)...)
	}

	var ifaceAddr string
	if len(config.BridgeIp) != 0 {
		_, _, err := net.ParseCIDR(config.BridgeIp)
		if err != nil {
			return err
		}
		ifaceAddr = config.BridgeIp
	} else {
		for _, addr := range addrs {
			_, dockerNetwork, err := net.ParseCIDR(addr)
			if err != nil {
				return err
			}
			if err := networkdriver.CheckNameserverOverlaps(nameservers, dockerNetwork); err == nil {
				if err := networkdriver.CheckRouteOverlaps(dockerNetwork); err == nil {
					ifaceAddr = addr
					break
				} else {
					utils.Debugf("%s %s", addr, err)
				}
			}
		}
	}

	if ifaceAddr == "" {
		return fmt.Errorf("Could not find a free IP address range for interface '%s'. Please configure its address manually and run 'docker -b %s'", config.BridgeIface, config.BridgeIface)
	}
	utils.Debugf("Creating bridge %s with network %s", config.BridgeIface, ifaceAddr)

	if err := createBridgeIface(config.BridgeIface); err != nil {
		return err
	}
	iface, err := net.InterfaceByName(config.BridgeIface)
	if err != nil {
		return err
	}
	ipAddr, ipNet, err := net.ParseCIDR(ifaceAddr)
	if err != nil {
		return err
	}
	if netlink.NetworkLinkAddIp(iface, ipAddr, ipNet); err != nil {
		return fmt.Errorf("Unable to add private network: %s", err)
	}
	if err := netlink.NetworkLinkUp(iface); err != nil {
		return fmt.Errorf("Unable to start network bridge: %s", err)
	}

	return nil
}

// Create the actual bridge device.  This is more backward-compatible than
// netlink.NetworkLinkAdd and works on RHEL 6.
func createBridgeIface(name string) error {
	s, err := syscall.Socket(syscall.AF_INET6, syscall.SOCK_STREAM, syscall.IPPROTO_IP)
	if err != nil {
		utils.Debugf("Bridge socket creation failed IPv6 probably not enabled: %v", err)
		s, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_IP)
		if err != nil {
			return fmt.Errorf("Error creating bridge creation socket: %s", err)
		}
	}
	defer syscall.Close(s)

	nameBytePtr, err := syscall.BytePtrFromString(name)
	if err != nil {
		return fmt.Errorf("Error converting bridge name %s to byte array: %s", name, err)
	}

	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(s), siocBRADDBR, uintptr(unsafe.Pointer(nameBytePtr))); err != 0 {
		return fmt.Errorf("Error creating bridge: %s", err)
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
	tcpMapping map[string]*net.TCPAddr
	tcpProxies map[string]proxy.Proxy
	udpMapping map[string]*net.UDPAddr
	udpProxies map[string]proxy.Proxy

	iptables         *iptables.Chain
	defaultIp        net.IP
	proxyFactoryFunc func(net.Addr, net.Addr) (proxy.Proxy, error)
}

func (mapper *PortMapper) Map(ip net.IP, port int, backendAddr net.Addr) error {

	if _, isTCP := backendAddr.(*net.TCPAddr); isTCP {
		mapKey := (&net.TCPAddr{Port: port, IP: ip}).String()
		if _, exists := mapper.tcpProxies[mapKey]; exists {
			return fmt.Errorf("TCP Port %s is already in use", mapKey)
		}
		backendPort := backendAddr.(*net.TCPAddr).Port
		backendIP := backendAddr.(*net.TCPAddr).IP
		if mapper.iptables != nil {
			if err := mapper.iptables.Forward(iptables.Add, ip, port, "tcp", backendIP.String(), backendPort); err != nil {
				return err
			}
		}
		mapper.tcpMapping[mapKey] = backendAddr.(*net.TCPAddr)
		proxy, err := mapper.proxyFactoryFunc(&net.TCPAddr{IP: ip, Port: port}, backendAddr)
		if err != nil {
			mapper.Unmap(ip, port, "tcp")
			return err
		}
		mapper.tcpProxies[mapKey] = proxy
		go proxy.Run()
	} else {
		mapKey := (&net.UDPAddr{Port: port, IP: ip}).String()
		if _, exists := mapper.udpProxies[mapKey]; exists {
			return fmt.Errorf("UDP: Port %s is already in use", mapKey)
		}
		backendPort := backendAddr.(*net.UDPAddr).Port
		backendIP := backendAddr.(*net.UDPAddr).IP
		if mapper.iptables != nil {
			if err := mapper.iptables.Forward(iptables.Add, ip, port, "udp", backendIP.String(), backendPort); err != nil {
				return err
			}
		}
		mapper.udpMapping[mapKey] = backendAddr.(*net.UDPAddr)
		proxy, err := mapper.proxyFactoryFunc(&net.UDPAddr{IP: ip, Port: port}, backendAddr)
		if err != nil {
			mapper.Unmap(ip, port, "udp")
			return err
		}
		mapper.udpProxies[mapKey] = proxy
		go proxy.Run()
	}
	return nil
}

func (mapper *PortMapper) Unmap(ip net.IP, port int, proto string) error {
	if proto == "tcp" {
		mapKey := (&net.TCPAddr{Port: port, IP: ip}).String()
		backendAddr, ok := mapper.tcpMapping[mapKey]
		if !ok {
			return fmt.Errorf("Port tcp/%s is not mapped", mapKey)
		}
		if proxy, exists := mapper.tcpProxies[mapKey]; exists {
			proxy.Close()
			delete(mapper.tcpProxies, mapKey)
		}
		if mapper.iptables != nil {
			if err := mapper.iptables.Forward(iptables.Delete, ip, port, proto, backendAddr.IP.String(), backendAddr.Port); err != nil {
				return err
			}
		}
		delete(mapper.tcpMapping, mapKey)
	} else {
		mapKey := (&net.UDPAddr{Port: port, IP: ip}).String()
		backendAddr, ok := mapper.udpMapping[mapKey]
		if !ok {
			return fmt.Errorf("Port udp/%s is not mapped", mapKey)
		}
		if proxy, exists := mapper.udpProxies[mapKey]; exists {
			proxy.Close()
			delete(mapper.udpProxies, mapKey)
		}
		if mapper.iptables != nil {
			if err := mapper.iptables.Forward(iptables.Delete, ip, port, proto, backendAddr.IP.String(), backendAddr.Port); err != nil {
				return err
			}
		}
		delete(mapper.udpMapping, mapKey)
	}
	return nil
}

func newPortMapper(config *DaemonConfig) (*PortMapper, error) {
	// We can always try removing the iptables
	if err := iptables.RemoveExistingChain("DOCKER"); err != nil {
		return nil, err
	}
	var chain *iptables.Chain
	if config.EnableIptables {
		var err error
		chain, err = iptables.NewChain("DOCKER", config.BridgeIface)
		if err != nil {
			return nil, fmt.Errorf("Failed to create DOCKER chain: %s", err)
		}
	}

	mapper := &PortMapper{
		tcpMapping:       make(map[string]*net.TCPAddr),
		tcpProxies:       make(map[string]proxy.Proxy),
		udpMapping:       make(map[string]*net.UDPAddr),
		udpProxies:       make(map[string]proxy.Proxy),
		iptables:         chain,
		defaultIp:        config.DefaultIp,
		proxyFactoryFunc: proxy.NewProxy,
	}
	return mapper, nil
}

// Port allocator: Automatically allocate and release networking ports
type PortAllocator struct {
	sync.Mutex
	inUse    map[string]struct{}
	fountain chan int
	quit     chan bool
}

func (alloc *PortAllocator) runFountain() {
	for {
		for port := portRangeStart; port < portRangeEnd; port++ {
			select {
			case alloc.fountain <- port:
			case quit := <-alloc.quit:
				if quit {
					return
				}
			}
		}
	}
}

// FIXME: Release can no longer fail, change its prototype to reflect that.
func (alloc *PortAllocator) Release(addr net.IP, port int) error {
	mapKey := (&net.TCPAddr{Port: port, IP: addr}).String()
	utils.Debugf("Releasing %d", port)
	alloc.Lock()
	delete(alloc.inUse, mapKey)
	alloc.Unlock()
	return nil
}

func (alloc *PortAllocator) Acquire(addr net.IP, port int) (int, error) {
	mapKey := (&net.TCPAddr{Port: port, IP: addr}).String()
	utils.Debugf("Acquiring %s", mapKey)
	if port == 0 {
		// Allocate a port from the fountain
		for port := range alloc.fountain {
			if _, err := alloc.Acquire(addr, port); err == nil {
				return port, nil
			}
		}
		return -1, fmt.Errorf("Port generator ended unexpectedly")
	}
	alloc.Lock()
	defer alloc.Unlock()
	if _, inUse := alloc.inUse[mapKey]; inUse {
		return -1, fmt.Errorf("Port already in use: %d", port)
	}
	alloc.inUse[mapKey] = struct{}{}
	return port, nil
}

func (alloc *PortAllocator) Close() error {
	alloc.quit <- true
	close(alloc.quit)
	close(alloc.fountain)
	return nil
}

func newPortAllocator() (*PortAllocator, error) {
	allocator := &PortAllocator{
		inUse:    make(map[string]struct{}),
		fountain: make(chan int),
		quit:     make(chan bool),
	}
	go allocator.runFountain()
	return allocator, nil
}

// Network interface represents the networking stack of a container
type NetworkInterface struct {
	IPNet   net.IPNet
	Gateway net.IP

	manager  *NetworkManager
	extPorts []*Nat
	disabled bool
}

// Allocate an external port and map it to the interface
func (iface *NetworkInterface) AllocatePort(port Port, binding PortBinding) (*Nat, error) {

	if iface.disabled {
		return nil, fmt.Errorf("Trying to allocate port for interface %v, which is disabled", iface) // FIXME
	}

	ip := iface.manager.portMapper.defaultIp

	if binding.HostIp != "" {
		ip = net.ParseIP(binding.HostIp)
	} else {
		binding.HostIp = ip.String()
	}

	nat := &Nat{
		Port:    port,
		Binding: binding,
	}

	containerPort, err := parsePort(port.Port())
	if err != nil {
		return nil, err
	}

	hostPort, _ := parsePort(nat.Binding.HostPort)

	if nat.Port.Proto() == "tcp" {
		extPort, err := iface.manager.tcpPortAllocator.Acquire(ip, hostPort)
		if err != nil {
			return nil, err
		}

		backend := &net.TCPAddr{IP: iface.IPNet.IP, Port: containerPort}
		if err := iface.manager.portMapper.Map(ip, extPort, backend); err != nil {
			iface.manager.tcpPortAllocator.Release(ip, extPort)
			return nil, err
		}
		nat.Binding.HostPort = strconv.Itoa(extPort)
	} else {
		extPort, err := iface.manager.udpPortAllocator.Acquire(ip, hostPort)
		if err != nil {
			return nil, err
		}
		backend := &net.UDPAddr{IP: iface.IPNet.IP, Port: containerPort}
		if err := iface.manager.portMapper.Map(ip, extPort, backend); err != nil {
			iface.manager.udpPortAllocator.Release(ip, extPort)
			return nil, err
		}
		nat.Binding.HostPort = strconv.Itoa(extPort)
	}
	iface.extPorts = append(iface.extPorts, nat)

	return nat, nil
}

type Nat struct {
	Port    Port
	Binding PortBinding
}

func (n *Nat) String() string {
	return fmt.Sprintf("%s:%s:%s/%s", n.Binding.HostIp, n.Binding.HostPort, n.Port.Port(), n.Port.Proto())
}

// Release: Network cleanup - release all resources
func (iface *NetworkInterface) Release() {
	if iface.disabled {
		return
	}

	for _, nat := range iface.extPorts {
		hostPort, err := parsePort(nat.Binding.HostPort)
		if err != nil {
			log.Printf("Unable to get host port: %s", err)
			continue
		}
		ip := net.ParseIP(nat.Binding.HostIp)
		utils.Debugf("Unmaping %s/%s:%s", nat.Port.Proto, ip.String(), nat.Binding.HostPort)
		if err := iface.manager.portMapper.Unmap(ip, hostPort, nat.Port.Proto()); err != nil {
			log.Printf("Unable to unmap port %s: %s", nat, err)
		}

		if nat.Port.Proto() == "tcp" {
			if err := iface.manager.tcpPortAllocator.Release(ip, hostPort); err != nil {
				log.Printf("Unable to release port %s", nat)
			}
		} else if nat.Port.Proto() == "udp" {
			if err := iface.manager.udpPortAllocator.Release(ip, hostPort); err != nil {
				log.Printf("Unable to release port %s: %s", nat, err)
			}
		}
	}

	if err := ipallocator.ReleaseIP(iface.manager.bridgeNetwork, &iface.IPNet.IP); err != nil {
		log.Printf("Unable to release ip %s\n", err)
	}
}

// Network Manager manages a set of network interfaces
// Only *one* manager per host machine should be used
type NetworkManager struct {
	bridgeIface   string
	bridgeNetwork *net.IPNet

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

	var ip *net.IP
	var err error

	ip, err = ipallocator.RequestIP(manager.bridgeNetwork, nil)
	if err != nil {
		return nil, err
	}

	iface := &NetworkInterface{
		IPNet:   net.IPNet{IP: *ip, Mask: manager.bridgeNetwork.Mask},
		Gateway: manager.bridgeNetwork.IP,
		manager: manager,
	}
	return iface, nil
}

func (manager *NetworkManager) Close() error {
	if manager.disabled {
		return nil
	}
	err1 := manager.tcpPortAllocator.Close()
	err2 := manager.udpPortAllocator.Close()
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return nil
}

func newNetworkManager(config *DaemonConfig) (*NetworkManager, error) {
	if config.BridgeIface == DisableNetworkBridge {
		manager := &NetworkManager{
			disabled: true,
		}
		return manager, nil
	}

	var network *net.IPNet
	addr, err := getIfaceAddr(config.BridgeIface)
	if err != nil {
		// If the iface is not found, try to create it
		if err := CreateBridgeIface(config); err != nil {
			return nil, err
		}
		addr, err = getIfaceAddr(config.BridgeIface)
		if err != nil {
			return nil, err
		}
		network = addr.(*net.IPNet)
	} else {
		network = addr.(*net.IPNet)
	}

	// Configure iptables for link support
	if config.EnableIptables {

		// Enable NAT
		natArgs := []string{"POSTROUTING", "-t", "nat", "-s", addr.String(), "!", "-d", addr.String(), "-j", "MASQUERADE"}

		if !iptables.Exists(natArgs...) {
			if output, err := iptables.Raw(append([]string{"-A"}, natArgs...)...); err != nil {
				return nil, fmt.Errorf("Unable to enable network bridge NAT: %s", err)
			} else if len(output) != 0 {
				return nil, fmt.Errorf("Error iptables postrouting: %s", output)
			}
		}

		// Accept incoming packets for existing connections
		existingArgs := []string{"FORWARD", "-o", config.BridgeIface, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"}

		if !iptables.Exists(existingArgs...) {
			if output, err := iptables.Raw(append([]string{"-I"}, existingArgs...)...); err != nil {
				return nil, fmt.Errorf("Unable to allow incoming packets: %s", err)
			} else if len(output) != 0 {
				return nil, fmt.Errorf("Error iptables allow incoming: %s", output)
			}
		}

		// Accept all non-intercontainer outgoing packets
		outgoingArgs := []string{"FORWARD", "-i", config.BridgeIface, "!", "-o", config.BridgeIface, "-j", "ACCEPT"}

		if !iptables.Exists(outgoingArgs...) {
			if output, err := iptables.Raw(append([]string{"-I"}, outgoingArgs...)...); err != nil {
				return nil, fmt.Errorf("Unable to allow outgoing packets: %s", err)
			} else if len(output) != 0 {
				return nil, fmt.Errorf("Error iptables allow outgoing: %s", output)
			}
		}

		args := []string{"FORWARD", "-i", config.BridgeIface, "-o", config.BridgeIface, "-j"}
		acceptArgs := append(args, "ACCEPT")
		dropArgs := append(args, "DROP")

		if !config.InterContainerCommunication {
			iptables.Raw(append([]string{"-D"}, acceptArgs...)...)
			if !iptables.Exists(dropArgs...) {
				utils.Debugf("Disable inter-container communication")
				if output, err := iptables.Raw(append([]string{"-I"}, dropArgs...)...); err != nil {
					return nil, fmt.Errorf("Unable to prevent intercontainer communication: %s", err)
				} else if len(output) != 0 {
					return nil, fmt.Errorf("Error disabling intercontainer communication: %s", output)
				}
			}
		} else {
			iptables.Raw(append([]string{"-D"}, dropArgs...)...)
			if !iptables.Exists(acceptArgs...) {
				utils.Debugf("Enable inter-container communication")
				if output, err := iptables.Raw(append([]string{"-I"}, acceptArgs...)...); err != nil {
					return nil, fmt.Errorf("Unable to allow intercontainer communication: %s", err)
				} else if len(output) != 0 {
					return nil, fmt.Errorf("Error enabling intercontainer communication: %s", output)
				}
			}
		}
	}

	tcpPortAllocator, err := newPortAllocator()
	if err != nil {
		return nil, err
	}

	udpPortAllocator, err := newPortAllocator()
	if err != nil {
		return nil, err
	}

	portMapper, err := newPortMapper(config)
	if err != nil {
		return nil, err
	}

	manager := &NetworkManager{
		bridgeIface:      config.BridgeIface,
		bridgeNetwork:    network,
		tcpPortAllocator: tcpPortAllocator,
		udpPortAllocator: udpPortAllocator,
		portMapper:       portMapper,
	}

	return manager, nil
}
