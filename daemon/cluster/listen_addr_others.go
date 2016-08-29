// +build !linux

package cluster

import "net"

func (c *Cluster) resolveSystemAddr() (net.IP, error) {
	// Use the system's only IP address, or fail if there are
	// multiple addresses to choose from.
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var systemAddr net.IP
	var systemInterface string

	// List Docker-managed subnets
	v4Subnets := c.config.NetworkSubnetsProvider.V4Subnets()
	v6Subnets := c.config.NetworkSubnetsProvider.V6Subnets()

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
