package bridge

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/network"
	"github.com/docker/docker/daemon/networkdriver"
	"github.com/docker/docker/daemon/networkdriver/ipallocator"
	"github.com/docker/docker/daemon/networkdriver/portmapper"
	"github.com/docker/docker/nat"
	"github.com/docker/docker/pkg/iptables"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/pkg/resolvconf"
	"github.com/docker/libcontainer/netlink"
)

const (
	DefaultNetworkBridge     = "docker0"
	MaxAllocatedPortAttempts = 10
)

// Network interface represents the networking stack of a container
type networkInterface struct {
	IP           net.IP
	IPv6         net.IP
	PortMappings []net.Addr // There are mappings to the host interfaces
}

type ifaces struct {
	c map[string]*networkInterface
	sync.Mutex
}

func (i *ifaces) Set(key string, n *networkInterface) {
	i.Lock()
	i.c[key] = n
	i.Unlock()
}

func (i *ifaces) Get(key string) *networkInterface {
	i.Lock()
	res := i.c[key]
	i.Unlock()
	return res
}

var (
	addrs = []string{
		// Here we don't follow the convention of using the 1st IP of the range for the gateway.
		// This is to use the same gateway IPs as the /24 ranges, which predate the /16 ranges.
		// In theory this shouldn't matter - in practice there's bound to be a few scripts relying
		// on the internal addressing or other things like that.
		// They shouldn't, but hey, let's not break them unless we really have to.
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

	bridgeIface       string
	bridgeIPv4Network *net.IPNet
	gatewayIPv4       net.IP
	bridgeIPv6Addr    net.IP
	globalIPv6Network *net.IPNet
	gatewayIPv6       net.IP
	portMapper        *portmapper.PortMapper
	once              sync.Once

	defaultBindingIP  = net.ParseIP("0.0.0.0")
	currentInterfaces = ifaces{c: make(map[string]*networkInterface)}
	ipAllocator       = ipallocator.New()
)

func initPortMapper() {
	once.Do(func() {
		portMapper = portmapper.New()
	})
}

type Config struct {
	EnableIPv6                  bool
	EnableIptables              bool
	EnableIpForward             bool
	EnableIpMasq                bool
	DefaultIp                   net.IP
	Iface                       string
	IP                          string
	FixedCIDR                   string
	FixedCIDRv6                 string
	DefaultGatewayIPv4          string
	DefaultGatewayIPv6          string
	InterContainerCommunication bool
}

func InitDriver(config *Config) error {
	var (
		networkv4  *net.IPNet
		networkv6  *net.IPNet
		addrv4     net.Addr
		addrsv6    []net.Addr
		bridgeIPv6 = "fe80::1/64"
	)

	// try to modprobe bridge first
	// see gh#12177
	if out, err := exec.Command("modprobe", "-va", "bridge", "nf_nat").Output(); err != nil {
		logrus.Warnf("Running modprobe bridge nf_nat failed with message: %s, error: %v", out, err)
	}

	initPortMapper()

	if config.DefaultIp != nil {
		defaultBindingIP = config.DefaultIp
	}

	bridgeIface = config.Iface
	usingDefaultBridge := false
	if bridgeIface == "" {
		usingDefaultBridge = true
		bridgeIface = DefaultNetworkBridge
	}

	addrv4, addrsv6, err := networkdriver.GetIfaceAddr(bridgeIface)

	if err != nil {
		// No Bridge existent, create one
		// If we're not using the default bridge, fail without trying to create it
		if !usingDefaultBridge {
			return err
		}

		logrus.Info("Bridge interface not found, trying to create it")

		// If the iface is not found, try to create it
		if err := configureBridge(config.IP, bridgeIPv6, config.EnableIPv6); err != nil {
			logrus.Errorf("Could not configure Bridge: %s", err)
			return err
		}

		addrv4, addrsv6, err = networkdriver.GetIfaceAddr(bridgeIface)
		if err != nil {
			return err
		}

		if config.FixedCIDRv6 != "" {
			// Setting route to global IPv6 subnet
			logrus.Infof("Adding route to IPv6 network %q via device %q", config.FixedCIDRv6, bridgeIface)
			if err := netlink.AddRoute(config.FixedCIDRv6, "", "", bridgeIface); err != nil {
				logrus.Fatalf("Could not add route to IPv6 network %q via device %q", config.FixedCIDRv6, bridgeIface)
			}
		}
	} else {
		// Bridge exists already, getting info...
		// Validate that the bridge ip matches the ip specified by BridgeIP
		if config.IP != "" {
			networkv4 = addrv4.(*net.IPNet)
			bip, _, err := net.ParseCIDR(config.IP)
			if err != nil {
				return err
			}
			if !networkv4.IP.Equal(bip) {
				return fmt.Errorf("Bridge ip (%s) does not match existing bridge configuration %s", networkv4.IP, bip)
			}
		}

		// A bridge might exist but not have any IPv6 addr associated with it yet
		// (for example, an existing Docker installation that has only been used
		// with IPv4 and docker0 already is set up) In that case, we can perform
		// the bridge init for IPv6 here, else we will error out below if --ipv6=true
		if len(addrsv6) == 0 && config.EnableIPv6 {
			if err := setupIPv6Bridge(bridgeIPv6); err != nil {
				return err
			}
			// Recheck addresses now that IPv6 is setup on the bridge
			addrv4, addrsv6, err = networkdriver.GetIfaceAddr(bridgeIface)
			if err != nil {
				return err
			}
		}

		// TODO: Check if route to config.FixedCIDRv6 is set
	}

	if config.EnableIPv6 {
		bip6, _, err := net.ParseCIDR(bridgeIPv6)
		if err != nil {
			return err
		}
		found := false
		for _, addrv6 := range addrsv6 {
			networkv6 = addrv6.(*net.IPNet)
			if networkv6.IP.Equal(bip6) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("Bridge IPv6 does not match existing bridge configuration %s", bip6)
		}
	}

	networkv4 = addrv4.(*net.IPNet)

	if config.EnableIPv6 {
		if len(addrsv6) == 0 {
			return errors.New("IPv6 enabled but no IPv6 detected")
		}
		bridgeIPv6Addr = networkv6.IP
	}

	// Configure iptables for link support
	if config.EnableIptables {
		if err := setupIPTables(addrv4, config.InterContainerCommunication, config.EnableIpMasq); err != nil {
			logrus.Errorf("Error configuing iptables: %s", err)
			return err
		}

	}

	if config.EnableIpForward {
		// Enable IPv4 forwarding
		if err := ioutil.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte{'1', '\n'}, 0644); err != nil {
			logrus.Warnf("WARNING: unable to enable IPv4 forwarding: %s\n", err)
		}

		if config.FixedCIDRv6 != "" {
			// Enable IPv6 forwarding
			if err := ioutil.WriteFile("/proc/sys/net/ipv6/conf/default/forwarding", []byte{'1', '\n'}, 0644); err != nil {
				logrus.Warnf("WARNING: unable to enable IPv6 default forwarding: %s\n", err)
			}
			if err := ioutil.WriteFile("/proc/sys/net/ipv6/conf/all/forwarding", []byte{'1', '\n'}, 0644); err != nil {
				logrus.Warnf("WARNING: unable to enable IPv6 all forwarding: %s\n", err)
			}
		}
	}

	// We can always try removing the iptables
	if err := iptables.RemoveExistingChain("DOCKER", iptables.Nat); err != nil {
		return err
	}

	if config.EnableIptables {
		_, err := iptables.NewChain("DOCKER", bridgeIface, iptables.Nat)
		if err != nil {
			return err
		}
		chain, err := iptables.NewChain("DOCKER", bridgeIface, iptables.Filter)
		if err != nil {
			return err
		}
		portMapper.SetIptablesChain(chain)
	}

	bridgeIPv4Network = networkv4
	if config.FixedCIDR != "" {
		_, subnet, err := net.ParseCIDR(config.FixedCIDR)
		if err != nil {
			return err
		}
		logrus.Debugf("Subnet: %v", subnet)
		if err := ipAllocator.RegisterSubnet(bridgeIPv4Network, subnet); err != nil {
			logrus.Errorf("Error registering subnet for IPv4 bridge network: %s", err)
			return err
		}
	}

	if gateway, err := requestDefaultGateway(config.DefaultGatewayIPv4, bridgeIPv4Network); err != nil {
		return err
	} else {
		gatewayIPv4 = gateway
	}

	if config.FixedCIDRv6 != "" {
		_, subnet, err := net.ParseCIDR(config.FixedCIDRv6)
		if err != nil {
			return err
		}
		logrus.Debugf("Subnet: %v", subnet)
		if err := ipAllocator.RegisterSubnet(subnet, subnet); err != nil {
			logrus.Errorf("Error registering subnet for IPv6 bridge network: %s", err)
			return err
		}
		globalIPv6Network = subnet

		if gateway, err := requestDefaultGateway(config.DefaultGatewayIPv6, globalIPv6Network); err != nil {
			return err
		} else {
			gatewayIPv6 = gateway
		}
	}

	// Block BridgeIP in IP allocator
	ipAllocator.RequestIP(bridgeIPv4Network, bridgeIPv4Network.IP)

	return nil
}

func setupIPTables(addr net.Addr, icc, ipmasq bool) error {
	// Enable NAT

	if ipmasq {
		natArgs := []string{"-s", addr.String(), "!", "-o", bridgeIface, "-j", "MASQUERADE"}

		if !iptables.Exists(iptables.Nat, "POSTROUTING", natArgs...) {
			if output, err := iptables.Raw(append([]string{
				"-t", string(iptables.Nat), "-I", "POSTROUTING"}, natArgs...)...); err != nil {
				return fmt.Errorf("Unable to enable network bridge NAT: %s", err)
			} else if len(output) != 0 {
				return iptables.ChainError{Chain: "POSTROUTING", Output: output}
			}
		}
	}

	var (
		args       = []string{"-i", bridgeIface, "-o", bridgeIface, "-j"}
		acceptArgs = append(args, "ACCEPT")
		dropArgs   = append(args, "DROP")
	)

	if !icc {
		iptables.Raw(append([]string{"-D", "FORWARD"}, acceptArgs...)...)

		if !iptables.Exists(iptables.Filter, "FORWARD", dropArgs...) {
			logrus.Debugf("Disable inter-container communication")
			if output, err := iptables.Raw(append([]string{"-I", "FORWARD"}, dropArgs...)...); err != nil {
				return fmt.Errorf("Unable to prevent intercontainer communication: %s", err)
			} else if len(output) != 0 {
				return fmt.Errorf("Error disabling intercontainer communication: %s", output)
			}
		}
	} else {
		iptables.Raw(append([]string{"-D", "FORWARD"}, dropArgs...)...)

		if !iptables.Exists(iptables.Filter, "FORWARD", acceptArgs...) {
			logrus.Debugf("Enable inter-container communication")
			if output, err := iptables.Raw(append([]string{"-I", "FORWARD"}, acceptArgs...)...); err != nil {
				return fmt.Errorf("Unable to allow intercontainer communication: %s", err)
			} else if len(output) != 0 {
				return fmt.Errorf("Error enabling intercontainer communication: %s", output)
			}
		}
	}

	// Accept all non-intercontainer outgoing packets
	outgoingArgs := []string{"-i", bridgeIface, "!", "-o", bridgeIface, "-j", "ACCEPT"}
	if !iptables.Exists(iptables.Filter, "FORWARD", outgoingArgs...) {
		if output, err := iptables.Raw(append([]string{"-I", "FORWARD"}, outgoingArgs...)...); err != nil {
			return fmt.Errorf("Unable to allow outgoing packets: %s", err)
		} else if len(output) != 0 {
			return iptables.ChainError{Chain: "FORWARD outgoing", Output: output}
		}
	}

	// Accept incoming packets for existing connections
	existingArgs := []string{"-o", bridgeIface, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"}

	if !iptables.Exists(iptables.Filter, "FORWARD", existingArgs...) {
		if output, err := iptables.Raw(append([]string{"-I", "FORWARD"}, existingArgs...)...); err != nil {
			return fmt.Errorf("Unable to allow incoming packets: %s", err)
		} else if len(output) != 0 {
			return iptables.ChainError{Chain: "FORWARD incoming", Output: output}
		}
	}
	return nil
}

func RequestPort(ip net.IP, proto string, port int) (int, error) {
	initPortMapper()
	return portMapper.Allocator.RequestPort(ip, proto, port)
}

// configureBridge attempts to create and configure a network bridge interface named `bridgeIface` on the host
// If bridgeIP is empty, it will try to find a non-conflicting IP from the Docker-specified private ranges
// If the bridge `bridgeIface` already exists, it will only perform the IP address association with the existing
// bridge (fixes issue #8444)
// If an address which doesn't conflict with existing interfaces can't be found, an error is returned.
func configureBridge(bridgeIP string, bridgeIPv6 string, enableIPv6 bool) error {
	nameservers := []string{}
	resolvConf, _ := resolvconf.Get()
	// We don't check for an error here, because we don't really care
	// if we can't read /etc/resolv.conf. So instead we skip the append
	// if resolvConf is nil. It either doesn't exist, or we can't read it
	// for some reason.
	if resolvConf != nil {
		nameservers = append(nameservers, resolvconf.GetNameserversAsCIDR(resolvConf)...)
	}

	var ifaceAddr string
	if len(bridgeIP) != 0 {
		_, _, err := net.ParseCIDR(bridgeIP)
		if err != nil {
			return err
		}
		ifaceAddr = bridgeIP
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
					logrus.Debugf("%s %s", addr, err)
				}
			}
		}
	}

	if ifaceAddr == "" {
		return fmt.Errorf("Could not find a free IP address range for interface '%s'. Please configure its address manually and run 'docker -b %s'", bridgeIface, bridgeIface)
	}
	logrus.Debugf("Creating bridge %s with network %s", bridgeIface, ifaceAddr)

	if err := createBridgeIface(bridgeIface); err != nil {
		// The bridge may already exist, therefore we can ignore an "exists" error
		if !os.IsExist(err) {
			return err
		}
	}

	iface, err := net.InterfaceByName(bridgeIface)
	if err != nil {
		return err
	}

	ipAddr, ipNet, err := net.ParseCIDR(ifaceAddr)
	if err != nil {
		return err
	}

	if err := netlink.NetworkLinkAddIp(iface, ipAddr, ipNet); err != nil {
		return fmt.Errorf("Unable to add private network: %s", err)
	}

	if enableIPv6 {
		if err := setupIPv6Bridge(bridgeIPv6); err != nil {
			return err
		}
	}

	if err := netlink.NetworkLinkUp(iface); err != nil {
		return fmt.Errorf("Unable to start network bridge: %s", err)
	}
	return nil
}

func setupIPv6Bridge(bridgeIPv6 string) error {

	iface, err := net.InterfaceByName(bridgeIface)
	if err != nil {
		return err
	}
	// Enable IPv6 on the bridge
	procFile := "/proc/sys/net/ipv6/conf/" + iface.Name + "/disable_ipv6"
	if err := ioutil.WriteFile(procFile, []byte{'0', '\n'}, 0644); err != nil {
		return fmt.Errorf("Unable to enable IPv6 addresses on bridge: %v", err)
	}

	ipAddr6, ipNet6, err := net.ParseCIDR(bridgeIPv6)
	if err != nil {
		return fmt.Errorf("Unable to parse bridge IPv6 address: %q, error: %v", bridgeIPv6, err)
	}

	if err := netlink.NetworkLinkAddIp(iface, ipAddr6, ipNet6); err != nil {
		return fmt.Errorf("Unable to add private IPv6 network: %v", err)
	}

	return nil
}

func requestDefaultGateway(requestedGateway string, network *net.IPNet) (gateway net.IP, err error) {
	if requestedGateway != "" {
		gateway = net.ParseIP(requestedGateway)

		if gateway == nil {
			return nil, fmt.Errorf("Bad parameter: invalid gateway ip %s", requestedGateway)
		}

		if !network.Contains(gateway) {
			return nil, fmt.Errorf("Gateway ip %s must be part of the network %s", requestedGateway, network.String())
		}

		ipAllocator.RequestIP(network, gateway)
	}

	return gateway, nil
}

func createBridgeIface(name string) error {
	kv, err := kernel.GetKernelVersion()
	// Only set the bridge's mac address if the kernel version is > 3.3
	// before that it was not supported
	setBridgeMacAddr := err == nil && (kv.Kernel >= 3 && kv.Major >= 3)
	logrus.Debugf("setting bridge mac address = %v", setBridgeMacAddr)
	return netlink.CreateBridge(name, setBridgeMacAddr)
}

// Generate a IEEE802 compliant MAC address from the given IP address.
//
// The generator is guaranteed to be consistent: the same IP will always yield the same
// MAC address. This is to avoid ARP cache issues.
func generateMacAddr(ip net.IP) net.HardwareAddr {
	hw := make(net.HardwareAddr, 6)

	// The first byte of the MAC address has to comply with these rules:
	// 1. Unicast: Set the least-significant bit to 0.
	// 2. Address is locally administered: Set the second-least-significant bit (U/L) to 1.
	// 3. As "small" as possible: The veth address has to be "smaller" than the bridge address.
	hw[0] = 0x02

	// The first 24 bits of the MAC represent the Organizationally Unique Identifier (OUI).
	// Since this address is locally administered, we can do whatever we want as long as
	// it doesn't conflict with other addresses.
	hw[1] = 0x42

	// Insert the IP address into the last 32 bits of the MAC address.
	// This is a simple way to guarantee the address will be consistent and unique.
	copy(hw[2:], ip.To4())

	return hw
}

func linkLocalIPv6FromMac(mac string) (string, error) {
	hx := strings.Replace(mac, ":", "", -1)
	hw, err := hex.DecodeString(hx)
	if err != nil {
		return "", errors.New("Could not parse MAC address " + mac)
	}

	hw[0] ^= 0x2

	return fmt.Sprintf("fe80::%x%x:%xff:fe%x:%x%x/64", hw[0], hw[1], hw[2], hw[3], hw[4], hw[5]), nil
}

// Allocate a network interface
func Allocate(id, requestedMac, requestedIP, requestedIPv6 string) (*network.Settings, error) {
	var (
		ip            net.IP
		mac           net.HardwareAddr
		err           error
		globalIPv6    net.IP
		defaultGWIPv4 net.IP
		defaultGWIPv6 net.IP
	)

	ip, err = ipAllocator.RequestIP(bridgeIPv4Network, net.ParseIP(requestedIP))
	if err != nil {
		return nil, err
	}

	// If no explicit mac address was given, generate a random one.
	if mac, err = net.ParseMAC(requestedMac); err != nil {
		mac = generateMacAddr(ip)
	}

	if globalIPv6Network != nil {
		// If globalIPv6Network Size is at least a /80 subnet generate IPv6 address from MAC address
		netmaskOnes, _ := globalIPv6Network.Mask.Size()
		ipv6 := net.ParseIP(requestedIPv6)
		if ipv6 == nil && netmaskOnes <= 80 {
			ipv6 = make(net.IP, len(globalIPv6Network.IP))
			copy(ipv6, globalIPv6Network.IP)
			for i, h := range mac {
				ipv6[i+10] = h
			}
		}

		globalIPv6, err = ipAllocator.RequestIP(globalIPv6Network, ipv6)
		if err != nil {
			logrus.Errorf("Allocator: RequestIP v6: %v", err)
			return nil, err
		}
		logrus.Infof("Allocated IPv6 %s", globalIPv6)
	}

	maskSize, _ := bridgeIPv4Network.Mask.Size()

	if gatewayIPv4 != nil {
		defaultGWIPv4 = gatewayIPv4
	} else {
		defaultGWIPv4 = bridgeIPv4Network.IP
	}

	if gatewayIPv6 != nil {
		defaultGWIPv6 = gatewayIPv6
	} else {
		defaultGWIPv6 = bridgeIPv6Addr
	}

	// If linklocal IPv6
	localIPv6Net, err := linkLocalIPv6FromMac(mac.String())
	if err != nil {
		return nil, err
	}
	localIPv6, _, _ := net.ParseCIDR(localIPv6Net)

	networkSettings := &network.Settings{
		IPAddress:            ip.String(),
		Gateway:              defaultGWIPv4.String(),
		MacAddress:           mac.String(),
		Bridge:               bridgeIface,
		IPPrefixLen:          maskSize,
		LinkLocalIPv6Address: localIPv6.String(),
	}

	if globalIPv6Network != nil {
		networkSettings.GlobalIPv6Address = globalIPv6.String()
		maskV6Size, _ := globalIPv6Network.Mask.Size()
		networkSettings.GlobalIPv6PrefixLen = maskV6Size
		networkSettings.IPv6Gateway = defaultGWIPv6.String()
	}

	currentInterfaces.Set(id, &networkInterface{
		IP:   ip,
		IPv6: globalIPv6,
	})

	return networkSettings, nil
}

// Release an interface for a select ip
func Release(id string) {
	var containerInterface = currentInterfaces.Get(id)

	if containerInterface == nil {
		logrus.Warnf("No network information to release for %s", id)
		return
	}

	for _, nat := range containerInterface.PortMappings {
		if err := portMapper.Unmap(nat); err != nil {
			logrus.Infof("Unable to unmap port %s: %s", nat, err)
		}
	}

	if err := ipAllocator.ReleaseIP(bridgeIPv4Network, containerInterface.IP); err != nil {
		logrus.Infof("Unable to release IPv4 %s", err)
	}
	if globalIPv6Network != nil {
		if err := ipAllocator.ReleaseIP(globalIPv6Network, containerInterface.IPv6); err != nil {
			logrus.Infof("Unable to release IPv6 %s", err)
		}
	}
}

// Allocate an external port and map it to the interface
func AllocatePort(id string, port nat.Port, binding nat.PortBinding) (nat.PortBinding, error) {
	var (
		ip            = defaultBindingIP
		proto         = port.Proto()
		containerPort = port.Int()
		network       = currentInterfaces.Get(id)
	)

	if binding.HostIp != "" {
		ip = net.ParseIP(binding.HostIp)
		if ip == nil {
			return nat.PortBinding{}, fmt.Errorf("Bad parameter: invalid host ip %s", binding.HostIp)
		}
	}

	// host ip, proto, and host port
	var container net.Addr
	switch proto {
	case "tcp":
		container = &net.TCPAddr{IP: network.IP, Port: containerPort}
	case "udp":
		container = &net.UDPAddr{IP: network.IP, Port: containerPort}
	default:
		return nat.PortBinding{}, fmt.Errorf("unsupported address type %s", proto)
	}

	//
	// Try up to 10 times to get a port that's not already allocated.
	//
	// In the event of failure to bind, return the error that portmapper.Map
	// yields.
	//

	var (
		host net.Addr
		err  error
	)
	hostPort, err := nat.ParsePort(binding.HostPort)
	if err != nil {
		return nat.PortBinding{}, err
	}
	for i := 0; i < MaxAllocatedPortAttempts; i++ {
		if host, err = portMapper.Map(container, ip, hostPort); err == nil {
			break
		}
		// There is no point in immediately retrying to map an explicitly
		// chosen port.
		if hostPort != 0 {
			logrus.Warnf("Failed to allocate and map port %d: %s", hostPort, err)
			break
		}
		logrus.Warnf("Failed to allocate and map port: %s, retry: %d", err, i+1)
	}

	if err != nil {
		return nat.PortBinding{}, err
	}

	network.PortMappings = append(network.PortMappings, host)

	switch netAddr := host.(type) {
	case *net.TCPAddr:
		return nat.PortBinding{HostIp: netAddr.IP.String(), HostPort: strconv.Itoa(netAddr.Port)}, nil
	case *net.UDPAddr:
		return nat.PortBinding{HostIp: netAddr.IP.String(), HostPort: strconv.Itoa(netAddr.Port)}, nil
	default:
		return nat.PortBinding{}, fmt.Errorf("unsupported address type %T", netAddr)
	}
}

//TODO: should it return something more than just an error?
func LinkContainers(action, parentIP, childIP string, ports []nat.Port, ignoreErrors bool) error {
	var nfAction iptables.Action

	switch action {
	case "-A":
		nfAction = iptables.Append
	case "-I":
		nfAction = iptables.Insert
	case "-D":
		nfAction = iptables.Delete
	default:
		return fmt.Errorf("Invalid action '%s' specified", action)
	}

	ip1 := net.ParseIP(parentIP)
	if ip1 == nil {
		return fmt.Errorf("Parent IP '%s' is invalid", parentIP)
	}
	ip2 := net.ParseIP(childIP)
	if ip2 == nil {
		return fmt.Errorf("Child IP '%s' is invalid", childIP)
	}

	chain := iptables.Chain{Name: "DOCKER", Bridge: bridgeIface}
	for _, port := range ports {
		if err := chain.Link(nfAction, ip1, ip2, port.Int(), port.Proto()); !ignoreErrors && err != nil {
			return err
		}
	}
	return nil
}
