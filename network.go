package docker

import (
	"fmt"
	"github.com/dotcloud/docker/networkdriver"
	"github.com/dotcloud/docker/networkdriver/ipallocator"
	"github.com/dotcloud/docker/networkdriver/portallocator"
	"github.com/dotcloud/docker/networkdriver/portmapper"
	"github.com/dotcloud/docker/pkg/iptables"
	"github.com/dotcloud/docker/pkg/netlink"
	"github.com/dotcloud/docker/utils"
	"io/ioutil"
	"log"
	"net"
	"strconv"
	"syscall"
	"unsafe"
)

const (
	DefaultNetworkBridge = "docker0"
	DisableNetworkBridge = "none"
	DefaultNetworkMtu    = 1500
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

	ip := iface.manager.defaultBindingIP

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

	extPort, err := portallocator.RequestPort(ip, nat.Port.Proto(), hostPort)
	if err != nil {
		return nil, err
	}

	var backend net.Addr
	if nat.Port.Proto() == "tcp" {
		backend = &net.TCPAddr{IP: iface.IPNet.IP, Port: containerPort}
	} else {
		backend = &net.UDPAddr{IP: iface.IPNet.IP, Port: containerPort}
	}

	if err := portmapper.Map(backend, ip, extPort); err != nil {
		portallocator.ReleasePort(ip, nat.Port.Proto(), extPort)
		return nil, err
	}

	nat.Binding.HostPort = strconv.Itoa(extPort)
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

		var host net.Addr
		if nat.Port.Proto() == "tcp" {
			host = &net.TCPAddr{IP: ip, Port: hostPort}
		} else {
			host = &net.UDPAddr{IP: ip, Port: hostPort}
		}

		if err := portmapper.Unmap(host); err != nil {
			log.Printf("Unable to unmap port %s: %s", nat, err)
		}

		if err := portallocator.ReleasePort(ip, nat.Port.Proto(), hostPort); err != nil {
			log.Printf("Unable to release port %s", nat)
		}
	}

	if err := ipallocator.ReleaseIP(iface.manager.bridgeNetwork, &iface.IPNet.IP); err != nil {
		log.Printf("Unable to release ip %s\n", err)
	}
}

// Network Manager manages a set of network interfaces
// Only *one* manager per host machine should be used
type NetworkManager struct {
	bridgeIface      string
	bridgeNetwork    *net.IPNet
	defaultBindingIP net.IP
	disabled         bool
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

		// Accept all non-intercontainer outgoing packets
		outgoingArgs := []string{"FORWARD", "-i", config.BridgeIface, "!", "-o", config.BridgeIface, "-j", "ACCEPT"}

		if !iptables.Exists(outgoingArgs...) {
			if output, err := iptables.Raw(append([]string{"-I"}, outgoingArgs...)...); err != nil {
				return nil, fmt.Errorf("Unable to allow outgoing packets: %s", err)
			} else if len(output) != 0 {
				return nil, fmt.Errorf("Error iptables allow outgoing: %s", output)
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

	}

	if config.EnableIpForward {
		// Enable IPv4 forwarding
		if err := ioutil.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte{'1', '\n'}, 0644); err != nil {
			log.Printf("WARNING: unable to enable IPv4 forwarding: %s\n", err)
		}
	}

	// We can always try removing the iptables
	if err := portmapper.RemoveIpTablesChain("DOCKER"); err != nil {
		return nil, err
	}

	if config.EnableIptables {
		if err := portmapper.RegisterIpTablesChain("DOCKER", config.BridgeIface); err != nil {
			return nil, err
		}
	}

	manager := &NetworkManager{
		bridgeIface:      config.BridgeIface,
		bridgeNetwork:    network,
		defaultBindingIP: config.DefaultIp,
	}
	return manager, nil
}
