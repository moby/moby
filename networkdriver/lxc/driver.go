package lxc

import (
	"fmt"
	"github.com/dotcloud/docker/engine"
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
	"strings"
	"syscall"
	"unsafe"
)

const (
	DefaultNetworkBridge = "docker0"
	siocBRADDBR          = 0x89a0
)

// Network interface represents the networking stack of a container
type networkInterface struct {
	IP           net.IP
	IP6          net.IP
	PortMappings []net.Addr // there are mappings to the host interfaces
}

var (
	addrs4 = []string{
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

	addrs6 = networkdriver.GenerateIPv6AddressPool()

	bridgeIface    string
	bridgeNetwork  *net.IPNet
	bridgeNetwork6 *net.IPNet

	defaultBindingIP  = net.ParseIP("0.0.0.0")
	defaultBindingIP6 = net.ParseIP("::")
	currentInterfaces = make(map[string]*networkInterface)
)

func init() {
	if err := engine.Register("init_networkdriver", InitDriver); err != nil {
		panic(err)
	}
}

func InitDriver(job *engine.Job) engine.Status {
	var (
		network        *net.IPNet
		network6       *net.IPNet
		enableIPTables = job.GetenvBool("EnableIptables")
		icc            = job.GetenvBool("InterContainerCommunication")
		ipForward      = job.GetenvBool("EnableIpForward")
		bridgeIP       = job.Getenv("BridgeIP")
		bridgeIP6      = job.Getenv("BridgeIP6")
	)

	if defaultIP := job.Getenv("DefaultBindingIP"); defaultIP != "" {
		defaultBindingIP = net.ParseIP(defaultIP)
	}
	if defaultIP6 := job.Getenv("DefaultBindingIP6"); defaultIP6 != "" {
		defaultBindingIP6 = net.ParseIP(defaultIP6)
	}

	bridgeIface = job.Getenv("BridgeIface")
	if bridgeIface == "" {
		bridgeIface = DefaultNetworkBridge
	}

	addr, err := networkdriver.GetIfaceAddr(bridgeIface)
	if err != nil {
		// If the iface is not found, try to create it
		job.Logf("creating new bridge for %s", bridgeIface)
		if err := createBridge(bridgeIP, bridgeIP6); err != nil {
			job.Error(err)
			return engine.StatusErr
		}

		job.Logf("getting iface addr")
		addr, err = networkdriver.GetIfaceAddr(bridgeIface)
		if err != nil {
			job.Error(err)
			return engine.StatusErr
		}
		network = addr.(*net.IPNet)
	} else {
		network = addr.(*net.IPNet)
	}

	addr6, err := networkdriver.GetIfaceAddr6(bridgeIface)
	if err != nil {
		// At this point we should have a bridge because of
		// IPv4. Throw an error
		job.Error(err)
		return engine.StatusErr
	} else {
		network6 = addr6.(*net.IPNet)
	}

	// Configure iptables for link support
	if enableIPTables {
		if err := setupIPTables(addr, icc); err != nil {
			job.Error(err)
			return engine.StatusErr
		}
	}

	if ipForward {
		// Enable IPv4 forwarding
		if err := ioutil.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte{'1', '\n'}, 0644); err != nil {
			job.Logf("WARNING: unable to enable IPv4 forwarding: %s\n", err)
		}

		// Enable IPv6 forwarding on all interfaces
		if err := ioutil.WriteFile("/proc/sys/net/ipv6/conf/all/forwarding", []byte{'1', '\n'}, 0644); err != nil {
			job.Logf("WARNING: unable to enable IPv6 forwarding: %s\n", err)
		}
	}

	// We can always try removing the iptables
	if err := iptables.RemoveExistingChain("DOCKER"); err != nil {
		job.Error(err)
		return engine.StatusErr
	}

	if enableIPTables {
		chain, err := iptables.NewChain("DOCKER", bridgeIface)
		if err != nil {
			job.Error(err)
			return engine.StatusErr
		}
		portmapper.SetIptablesChain(chain)
	}

	bridgeNetwork  = network
	bridgeNetwork6 = network6

	// https://github.com/dotcloud/docker/issues/2768
	job.Eng.Hack_SetGlobalVar("httpapi.bridgeIP", bridgeNetwork.IP)

	for name, f := range map[string]engine.Handler{
		"allocate_interface": Allocate,
		"release_interface":  Release,
		"allocate_port":      AllocatePort,
		"link":               LinkContainers,
	} {
		if err := job.Eng.Register(name, f); err != nil {
			job.Error(err)
			return engine.StatusErr
		}
	}
	return engine.StatusOK
}

func setupIPTables(addr net.Addr, icc bool) error {
	// Enable NAT
	natArgs := []string{"POSTROUTING", "-t", "nat", "-s", addr.String(), "!", "-d", addr.String(), "-j", "MASQUERADE"}

	if !iptables.Exists(natArgs...) {
		if output, err := iptables.Raw(append([]string{"-I"}, natArgs...)...); err != nil {
			return fmt.Errorf("Unable to enable network bridge NAT: %s", err)
		} else if len(output) != 0 {
			return fmt.Errorf("Error iptables postrouting: %s", output)
		}
	}

	var (
		args       = []string{"FORWARD", "-i", bridgeIface, "-o", bridgeIface, "-j"}
		acceptArgs = append(args, "ACCEPT")
		dropArgs   = append(args, "DROP")
	)

	if !icc {
		iptables.Raw(append([]string{"-D"}, acceptArgs...)...)

		if !iptables.Exists(dropArgs...) {
			utils.Debugf("Disable inter-container communication")
			if output, err := iptables.Raw(append([]string{"-I"}, dropArgs...)...); err != nil {
				return fmt.Errorf("Unable to prevent intercontainer communication: %s", err)
			} else if len(output) != 0 {
				return fmt.Errorf("Error disabling intercontainer communication: %s", output)
			}
		}
	} else {
		iptables.Raw(append([]string{"-D"}, dropArgs...)...)

		if !iptables.Exists(acceptArgs...) {
			utils.Debugf("Enable inter-container communication")
			if output, err := iptables.Raw(append([]string{"-I"}, acceptArgs...)...); err != nil {
				return fmt.Errorf("Unable to allow intercontainer communication: %s", err)
			} else if len(output) != 0 {
				return fmt.Errorf("Error enabling intercontainer communication: %s", output)
			}
		}
	}

	// Accept all non-intercontainer outgoing packets
	outgoingArgs := []string{"FORWARD", "-i", bridgeIface, "!", "-o", bridgeIface, "-j", "ACCEPT"}
	if !iptables.Exists(outgoingArgs...) {
		if output, err := iptables.Raw(append([]string{"-I"}, outgoingArgs...)...); err != nil {
			return fmt.Errorf("Unable to allow outgoing packets: %s", err)
		} else if len(output) != 0 {
			return fmt.Errorf("Error iptables allow outgoing: %s", output)
		}
	}

	// Accept incoming packets for existing connections
	existingArgs := []string{"FORWARD", "-o", bridgeIface, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"}

	if !iptables.Exists(existingArgs...) {
		if output, err := iptables.Raw(append([]string{"-I"}, existingArgs...)...); err != nil {
			return fmt.Errorf("Unable to allow incoming packets: %s", err)
		} else if len(output) != 0 {
			return fmt.Errorf("Error iptables allow incoming: %s", output)
		}
	}
	return nil
}

func findBridgeNetwork(preferredIp string, address_pool []string) (string, error) {
	nameservers := []string{}
	resolvConf, _ := utils.GetResolvConf()

	firstIP,_,_ := net.ParseCIDR(address_pool[0])

	// we don't check for an error here, because we don't really care
	// if we can't read /etc/resolv.conf. So instead we skip the append
	// if resolvConf is nil. It either doesn't exist, or we can't read it
	// for some reason.
	if resolvConf != nil {
		if !utils.IsIPv6(&firstIP) {
			nameservers = append(nameservers, utils.GetIPv4NameserversAsCIDR(resolvConf)...)
		} else {
			nameservers = append(nameservers, utils.GetIPv6NameserversAsCIDR(resolvConf)...)
		}
	}

	var ifaceAddr string
	if len(preferredIp) != 0 {
		_, _, err := net.ParseCIDR(preferredIp)
		if err != nil {
			return "", err
		}
		return preferredIp, nil
	} else {
		for _, addr := range address_pool {
			_, dockerNetwork, err := net.ParseCIDR(addr)
			if err != nil {
				return "", err
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
		return "", fmt.Errorf("Could not find a free IP address range for interface '%s'. Please configure its address manually and run 'docker -b %s'", bridgeIface, bridgeIface)
	}
	return ifaceAddr, nil
}

// CreateBridgeIface creates a network bridge interface on the host system with the name `ifaceName`,
// and attempts to configure it with an address which doesn't conflict with any other interface on the host.
// If it can't find an address which doesn't conflict, it will return an error.
func createBridge(bridgeIP, bridgeIP6 string) error {
	var inet, inet6 string

	inet, err := findBridgeNetwork(bridgeIP, addrs4)
	if err != nil {
		return err
	}
	inet6, err = findBridgeNetwork(bridgeIP6, addrs6)
	if err != nil {
		return err
	}


	utils.Debugf("Creating bridge %s with networks %s, %s", bridgeIface, inet, inet6)
	if err := createBridgeIface(bridgeIface); err != nil {
		return err
	}

	iface, err := net.InterfaceByName(bridgeIface)
	if err != nil {
		return err
	}

	ipAddr, ipNet, err := net.ParseCIDR(inet)
	if err != nil {
		return err
	}

	if netlink.NetworkLinkAddIp(iface, ipAddr, ipNet); err != nil {
		return fmt.Errorf("Unable to add private IPv4 network: %s", err)
	}

	ipAddr6, ipNet6, err := net.ParseCIDR(inet6)
	if err != nil {
		return err
	}

	if netlink.NetworkLinkAddIp(iface, ipAddr6, ipNet6); err != nil {
		return fmt.Errorf("Unable to add private IPv6 network: %s", err)
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

// Allocate a network interface
func Allocate(job *engine.Job) engine.Status {
	var (
		ip            *net.IP
		ip6           *net.IP
		err           error
		id            = job.Args[0]
		requestedIP   = net.ParseIP(job.Getenv("RequestedIP"))
		requestedIP6  = net.ParseIP(job.Getenv("RequestedIP6"))
	)

	// IPv4
	if requestedIP != nil {
		ip, err = ipallocator.RequestIP(bridgeNetwork, &requestedIP)
	} else {
		ip, err = ipallocator.RequestIP(bridgeNetwork, nil)
	}
	if err != nil {
		job.Error(err)
		return engine.StatusErr
	}
	// IPv6
	if requestedIP6 != nil {
		ip6, err = ipallocator.RequestIP(bridgeNetwork6, &requestedIP)
	} else {
		ip6, err = ipallocator.RequestIP(bridgeNetwork6, nil)
	}
	if err != nil {
		job.Error(err)
		return engine.StatusErr
	}

	out := engine.Env{}
	out.Set("IP", ip.String())
	out.Set("IP6", ip6.String())
	out.Set("Mask", bridgeNetwork.Mask.String())
	out.Set("Mask6", bridgeNetwork6.Mask.String())
	out.Set("Gateway", bridgeNetwork.IP.String())
	out.Set("Gateway6", bridgeNetwork6.IP.String())
	out.Set("Bridge", bridgeIface)

	// IPv4
	size, _ := bridgeNetwork.Mask.Size()
	out.SetInt("IPPrefixLen", size)
	// IPv6
	size6, _ := bridgeNetwork6.Mask.Size()
	out.SetInt("IPPrefixLen6", size6)

	currentInterfaces[id] = &networkInterface{
		IP:   *ip,
		IP6:  *ip6,
	}

	out.WriteTo(job.Stdout)

	return engine.StatusOK
}

// release an interface for a select ip
func Release(job *engine.Job) engine.Status {
	var (
		id                 = job.Args[0]
		containerInterface = currentInterfaces[id]
		ip                 net.IP
		port               int
		proto              string
	)

	if containerInterface == nil {
		return job.Errorf("No network information to release for %s", id)
	}

	for _, nat := range containerInterface.PortMappings {
		if err := portmapper.Unmap(nat); err != nil {
			log.Printf("Unable to unmap port %s: %s", nat, err)
		}

		// this is host mappings
		switch a := nat.(type) {
		case *net.TCPAddr:
			proto = "tcp"
			ip = a.IP
			port = a.Port
		case *net.UDPAddr:
			proto = "udp"
			ip = a.IP
			port = a.Port
		}

		if err := portallocator.ReleasePort(ip, proto, port); err != nil {
			log.Printf("Unable to release port %s", nat)
		}
	}

	if err := ipallocator.ReleaseIP(bridgeNetwork, &containerInterface.IP); err != nil {
		log.Printf("Unable to release ip %s\n", err)
	}
	if err := ipallocator.ReleaseIP(bridgeNetwork6, &containerInterface.IP6); err != nil {
		log.Printf("Unable to release ip %s\n", err)
	}
	return engine.StatusOK
}

// Allocate an external port and map it to the interface
func AllocatePort(job *engine.Job) engine.Status {
	var (
		err error

		// XXX: For now if the jobs hostIP is empty we just assume IPv4
		ip            = defaultBindingIP
		id            = job.Args[0]
		hostIP        = job.Getenv("HostIP")
		hostPort      = job.GetenvInt("HostPort")
		containerPort = job.GetenvInt("ContainerPort")
		proto         = job.Getenv("Proto")
		network       = currentInterfaces[id]
	)

	if hostIP != "" {
		ip = net.ParseIP(hostIP)
	}

	// host ip, proto, and host port
	hostPort, err = portallocator.RequestPort(ip, proto, hostPort)
	if err != nil {
		job.Error(err)
		return engine.StatusErr
	}

	var (
		container net.Addr
		host      net.Addr
	)

	if proto == "tcp" {
		host = &net.TCPAddr{IP: ip, Port: hostPort}
		if !utils.IsIPv6(&ip) {
			container = &net.TCPAddr{IP: network.IP, Port: containerPort}
		} else {
			container = &net.TCPAddr{IP: network.IP6, Port: containerPort}
		}
	} else {
		host = &net.UDPAddr{IP: ip, Port: hostPort}
		if !utils.IsIPv6(&ip) {
			container = &net.UDPAddr{IP: network.IP, Port: containerPort}
		} else {
			container = &net.UDPAddr{IP: network.IP6, Port: containerPort}
		}
	}

	if err := portmapper.Map(container, ip, hostPort); err != nil {
		portallocator.ReleasePort(ip, proto, hostPort)

		job.Error(err)
		return engine.StatusErr
	}
	network.PortMappings = append(network.PortMappings, host)

	out := engine.Env{}
	out.Set("HostIP", ip.String())
	out.SetInt("HostPort", hostPort)

	if _, err := out.WriteTo(job.Stdout); err != nil {
		job.Error(err)
		return engine.StatusErr
	}
	return engine.StatusOK
}

func LinkContainers(job *engine.Job) engine.Status {
	var (
		action       = job.Args[0]
		childIP      = job.Getenv("ChildIP")
		parentIP     = job.Getenv("ParentIP")
		ignoreErrors = job.GetenvBool("IgnoreErrors")
		ports        = job.GetenvList("Ports")
	)

	split := func(p string) (string, string) {
		parts := strings.Split(p, "/")
		return parts[0], parts[1]
	}

	for _, p := range ports {
		port, proto := split(p)

		if output, err := iptables.Raw(action, "FORWARD",
			"-i", bridgeIface, "-o", bridgeIface,
			"-p", proto,
			"-s", parentIP,
			"--dport", port,
			"-d", childIP,
			"-j", "ACCEPT"); !ignoreErrors && err != nil {
			job.Error(err)
			return engine.StatusErr
		} else if len(output) != 0 {
			job.Errorf("Error toggle iptables forward: %s", output)
			return engine.StatusErr
		}

		if output, err := iptables.Raw(action, "FORWARD",
			"-i", bridgeIface, "-o", bridgeIface,
			"-p", proto,
			"-s", childIP,
			"--sport", port,
			"-d", parentIP,
			"-j", "ACCEPT"); !ignoreErrors && err != nil {
			job.Error(err)
			return engine.StatusErr
		} else if len(output) != 0 {
			job.Errorf("Error toggle iptables forward: %s", output)
			return engine.StatusErr
		}
	}
	return engine.StatusOK
}
