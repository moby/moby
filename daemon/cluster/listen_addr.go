package cluster // import "github.com/moby/moby/daemon/cluster"

import (
	"fmt"
	"net"
	"strings"
)

const (
	errNoSuchInterface         configError = "no such interface"
	errNoIP                    configError = "could not find the system's IP address"
	errMustSpecifyListenAddr   configError = "must specify a listening address because the address to advertise is not recognized as a system address, and a system's IP address to use could not be uniquely identified"
	errBadNetworkIdentifier    configError = "must specify a valid IP address or interface name"
	errBadListenAddr           configError = "listen address must be an IP address or network interface (with optional port number)"
	errBadAdvertiseAddr        configError = "advertise address must be a non-zero IP address or network interface (with optional port number)"
	errBadDataPathAddr         configError = "data path address must be a non-zero IP address or network interface (without a port number)"
	errBadDefaultAdvertiseAddr configError = "default advertise address must be a non-zero IP address or network interface (without a port number)"
)

func resolveListenAddr(specifiedAddr string) (string, string, error) {
	specifiedHost, specifiedPort, err := net.SplitHostPort(specifiedAddr)
	if err != nil {
		return "", "", fmt.Errorf("could not parse listen address %s", specifiedAddr)
	}
	// Does the host component match any of the interface names on the
	// system? If so, use the address from that interface.
	specifiedIP, err := resolveInputIPAddr(specifiedHost, true)
	if err != nil {
		if err == errBadNetworkIdentifier {
			err = errBadListenAddr
		}
		return "", "", err
	}

	return specifiedIP.String(), specifiedPort, nil
}

func (c *Cluster) resolveAdvertiseAddr(advertiseAddr, listenAddrPort string) (string, string, error) {
	// Approach:
	// - If an advertise address is specified, use that. Resolve the
	//   interface's address if an interface was specified in
	//   advertiseAddr. Fill in the port from listenAddrPort if necessary.
	// - If DefaultAdvertiseAddr is not empty, use that with the port from
	//   listenAddrPort. Resolve the interface's address from
	//   if an interface name was specified in DefaultAdvertiseAddr.
	// - Otherwise, try to autodetect the system's address. Use the port in
	//   listenAddrPort with this address if autodetection succeeds.

	if advertiseAddr != "" {
		advertiseHost, advertisePort, err := net.SplitHostPort(advertiseAddr)
		if err != nil {
			// Not a host:port specification
			advertiseHost = advertiseAddr
			advertisePort = listenAddrPort
		}
		// Does the host component match any of the interface names on the
		// system? If so, use the address from that interface.
		advertiseIP, err := resolveInputIPAddr(advertiseHost, false)
		if err != nil {
			if err == errBadNetworkIdentifier {
				err = errBadAdvertiseAddr
			}
			return "", "", err
		}

		return advertiseIP.String(), advertisePort, nil
	}

	if c.config.DefaultAdvertiseAddr != "" {
		// Does the default advertise address component match any of the
		// interface names on the system? If so, use the address from
		// that interface.
		defaultAdvertiseIP, err := resolveInputIPAddr(c.config.DefaultAdvertiseAddr, false)
		if err != nil {
			if err == errBadNetworkIdentifier {
				err = errBadDefaultAdvertiseAddr
			}
			return "", "", err
		}

		return defaultAdvertiseIP.String(), listenAddrPort, nil
	}

	systemAddr, err := c.resolveSystemAddr()
	if err != nil {
		return "", "", err
	}
	return systemAddr.String(), listenAddrPort, nil
}

// validateDefaultAddrPool validates default address pool
// it also strips white space from the string before validation
func validateDefaultAddrPool(defaultAddrPool []string, size uint32) error {
	if defaultAddrPool == nil {
		// defaultAddrPool is not defined
		return nil
	}
	// if size is not set, then we use default value 24
	if size == 0 {
		size = 24
	}
	// We allow max value as 29. We can have 8 IP addresses for max value 29
	// If we allow 30, then we will get only 4 IP addresses. But with latest
	// libnetwork LB scale implementation, we use total of 4 IP addresses for internal use.
	// Hence keeping 29 as max value, we will have 8 IP addresses. This will be
	// smallest subnet that can be used in overlay network.
	if size > 29 {
		return fmt.Errorf("subnet size is out of range: %d", size)
	}
	for i := range defaultAddrPool {
		// trim leading and trailing white spaces
		defaultAddrPool[i] = strings.TrimSpace(defaultAddrPool[i])
		_, b, err := net.ParseCIDR(defaultAddrPool[i])
		if err != nil {
			return fmt.Errorf("invalid base pool %s: %v", defaultAddrPool[i], err)
		}
		ones, _ := b.Mask.Size()
		if size < uint32(ones) {
			return fmt.Errorf("invalid CIDR: %q. Subnet size is too small for pool: %d", defaultAddrPool[i], size)
		}
	}

	return nil
}

// getDataPathPort validates vxlan udp port (data path port) number.
// if no port is set, the default (4789) is returned
// valid port numbers are between 1024 and 49151
func getDataPathPort(portNum uint32) (uint32, error) {
	// if the value comes as 0 by any reason we set it to default value 4789
	if portNum == 0 {
		portNum = 4789
		return portNum, nil
	}
	// IANA procedures for each range in detail
	// The Well Known Ports, aka the System Ports, from 0-1023
	// The Registered Ports, aka the User Ports, from 1024-49151
	// The Dynamic Ports, aka the Private Ports, from 49152-65535
	// So we can allow range between 1024 to 49151
	if portNum < 1024 || portNum > 49151 {
		return 0, fmt.Errorf("Datapath port number is not in valid range (1024-49151) : %d", portNum)
	}
	return portNum, nil
}
func resolveDataPathAddr(dataPathAddr string) (string, error) {
	if dataPathAddr == "" {
		// dataPathAddr is not defined
		return "", nil
	}
	// If a data path flag is specified try to resolve the IP address.
	dataPathIP, err := resolveInputIPAddr(dataPathAddr, false)
	if err != nil {
		if err == errBadNetworkIdentifier {
			err = errBadDataPathAddr
		}
		return "", err
	}
	return dataPathIP.String(), nil
}

func resolveInterfaceAddr(specifiedInterface string) (net.IP, error) {
	// Use a specific interface's IP address.
	intf, err := net.InterfaceByName(specifiedInterface)
	if err != nil {
		return nil, errNoSuchInterface
	}

	addrs, err := intf.Addrs()
	if err != nil {
		return nil, err
	}

	var interfaceAddr4, interfaceAddr6 net.IP

	for _, addr := range addrs {
		ipAddr, ok := addr.(*net.IPNet)

		if ok {
			if ipAddr.IP.To4() != nil {
				// IPv4
				if interfaceAddr4 != nil {
					return nil, configError(fmt.Sprintf("interface %s has more than one IPv4 address (%s and %s)", specifiedInterface, interfaceAddr4, ipAddr.IP))
				}
				interfaceAddr4 = ipAddr.IP
			} else {
				// IPv6
				if interfaceAddr6 != nil {
					return nil, configError(fmt.Sprintf("interface %s has more than one IPv6 address (%s and %s)", specifiedInterface, interfaceAddr6, ipAddr.IP))
				}
				interfaceAddr6 = ipAddr.IP
			}
		}
	}

	if interfaceAddr4 == nil && interfaceAddr6 == nil {
		return nil, configError(fmt.Sprintf("interface %s has no usable IPv4 or IPv6 address", specifiedInterface))
	}

	// In the case that there's exactly one IPv4 address
	// and exactly one IPv6 address, favor IPv4 over IPv6.
	if interfaceAddr4 != nil {
		return interfaceAddr4, nil
	}
	return interfaceAddr6, nil
}

// resolveInputIPAddr tries to resolve the IP address from the string passed as input
// - tries to match the string as an interface name, if so returns the IP address associated with it
// - on failure of previous step tries to parse the string as an IP address itself
//	 if succeeds returns the IP address
func resolveInputIPAddr(input string, isUnspecifiedValid bool) (net.IP, error) {
	// Try to see if it is an interface name
	interfaceAddr, err := resolveInterfaceAddr(input)
	if err == nil {
		return interfaceAddr, nil
	}
	// String matched interface but there is a potential ambiguity to be resolved
	if err != errNoSuchInterface {
		return nil, err
	}

	// String is not an interface check if it is a valid IP
	if ip := net.ParseIP(input); ip != nil && (isUnspecifiedValid || !ip.IsUnspecified()) {
		return ip, nil
	}

	// Not valid IP found
	return nil, errBadNetworkIdentifier
}

func (c *Cluster) resolveSystemAddrViaSubnetCheck() (net.IP, error) {
	// Use the system's only IP address, or fail if there are
	// multiple addresses to choose from. Skip interfaces which
	// are managed by docker via subnet check.
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var systemAddr net.IP
	var systemInterface string

	// List Docker-managed subnets
	v4Subnets, v6Subnets := c.config.NetworkSubnetsProvider.Subnets()

ifaceLoop:
	for _, intf := range interfaces {
		// Skip inactive interfaces and loopback interfaces
		if (intf.Flags&net.FlagUp == 0) || (intf.Flags&net.FlagLoopback) != 0 {
			continue
		}

		addrs, err := intf.Addrs()
		if err != nil {
			continue
		}

		var interfaceAddr4, interfaceAddr6 net.IP

		for _, addr := range addrs {
			ipAddr, ok := addr.(*net.IPNet)

			// Skip loopback and link-local addresses
			if !ok || !ipAddr.IP.IsGlobalUnicast() {
				continue
			}

			if ipAddr.IP.To4() != nil {
				// IPv4

				// Ignore addresses in subnets that are managed by Docker.
				for _, subnet := range v4Subnets {
					if subnet.Contains(ipAddr.IP) {
						continue ifaceLoop
					}
				}

				if interfaceAddr4 != nil {
					return nil, errMultipleIPs(intf.Name, intf.Name, interfaceAddr4, ipAddr.IP)
				}

				interfaceAddr4 = ipAddr.IP
			} else {
				// IPv6

				// Ignore addresses in subnets that are managed by Docker.
				for _, subnet := range v6Subnets {
					if subnet.Contains(ipAddr.IP) {
						continue ifaceLoop
					}
				}

				if interfaceAddr6 != nil {
					return nil, errMultipleIPs(intf.Name, intf.Name, interfaceAddr6, ipAddr.IP)
				}

				interfaceAddr6 = ipAddr.IP
			}
		}

		// In the case that this interface has exactly one IPv4 address
		// and exactly one IPv6 address, favor IPv4 over IPv6.
		if interfaceAddr4 != nil {
			if systemAddr != nil {
				return nil, errMultipleIPs(systemInterface, intf.Name, systemAddr, interfaceAddr4)
			}
			systemAddr = interfaceAddr4
			systemInterface = intf.Name
		} else if interfaceAddr6 != nil {
			if systemAddr != nil {
				return nil, errMultipleIPs(systemInterface, intf.Name, systemAddr, interfaceAddr6)
			}
			systemAddr = interfaceAddr6
			systemInterface = intf.Name
		}
	}

	if systemAddr == nil {
		return nil, errNoIP
	}

	return systemAddr, nil
}

func listSystemIPs() []net.IP {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	var systemAddrs []net.IP

	for _, intf := range interfaces {
		addrs, err := intf.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipAddr, ok := addr.(*net.IPNet)

			if ok {
				systemAddrs = append(systemAddrs, ipAddr.IP)
			}
		}
	}

	return systemAddrs
}

func errMultipleIPs(interfaceA, interfaceB string, addrA, addrB net.IP) error {
	if interfaceA == interfaceB {
		return configError(fmt.Sprintf("could not choose an IP address to advertise since this system has multiple addresses on interface %s (%s and %s)", interfaceA, addrA, addrB))
	}
	return configError(fmt.Sprintf("could not choose an IP address to advertise since this system has multiple addresses on different interfaces (%s on %s and %s on %s)", addrA, interfaceA, addrB, interfaceB))
}
