package bridge

import (
	"fmt"
	"net"

	log "github.com/Sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

var bridgeNetworks []*net.IPNet

func init() {
	// Here we don't follow the convention of using the 1st IP of the range for the gateway.
	// This is to use the same gateway IPs as the /24 ranges, which predate the /16 ranges.
	// In theory this shouldn't matter - in practice there's bound to be a few scripts relying
	// on the internal addressing or other stupid things like that.
	// They shouldn't, but hey, let's not break them unless we really have to.
	for _, addr := range []string{
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
	} {
		ip, net, err := net.ParseCIDR(addr)
		if err != nil {
			log.Errorf("Failed to parse address %s", addr)
			continue
		}
		net.IP = ip
		bridgeNetworks = append(bridgeNetworks, net)
	}
}

func SetupBridgeIPv4(i *Interface) error {
	bridgeIPv4, err := electBridgeIPv4(i.Config)
	if err != nil {
		return err
	}

	log.Debugf("Creating bridge interface %q with network %s", i.Config.BridgeName, bridgeIPv4)
	if err := netlink.AddrAdd(i.Link, &netlink.Addr{bridgeIPv4, ""}); err != nil {
		return fmt.Errorf("Failed to add IPv4 address %s to bridge: %v", bridgeIPv4, err)
	}

	return nil
}

func electBridgeIPv4(config *Configuration) (*net.IPNet, error) {
	// Use the requested IPv4 CIDR when available.
	if config.AddressIPv4 != nil {
		return config.AddressIPv4, nil
	}

	// We don't check for an error here, because we don't really care if we
	// can't read /etc/resolv.conf. So instead we skip the append if resolvConf
	// is nil. It either doesn't exist, or we can't read it for some reason.
	nameservers := []string{}
	if resolvConf, _ := readResolvConf(); resolvConf != nil {
		nameservers = append(nameservers, getNameserversAsCIDR(resolvConf)...)
	}

	// Try to automatically elect appropriate brige IPv4 settings.
	for _, n := range bridgeNetworks {
		if err := checkNameserverOverlaps(nameservers, n); err == nil {
			if err := checkRouteOverlaps(n); err == nil {
				return n, nil
			}
		}
	}

	return nil, fmt.Errorf("Couldn't find an address range for interface %q", config.BridgeName)
}

func checkNameserverOverlaps(nameservers []string, toCheck *net.IPNet) error {
	for _, ns := range nameservers {
		_, nsNetwork, err := net.ParseCIDR(ns)
		if err != nil {
			return err
		}
		if networkOverlaps(toCheck, nsNetwork) {
			return fmt.Errorf("Requested network %s overlaps with name server")
		}
	}
	return nil
}

func checkRouteOverlaps(toCheck *net.IPNet) error {
	networks, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return err
	}

	for _, network := range networks {
		// TODO Is that right?
		if network.Dst != nil && networkOverlaps(toCheck, network.Dst) {
			return fmt.Errorf("Requested network %s overlaps with an existing network")
		}
	}
	return nil
}

func networkOverlaps(netX *net.IPNet, netY *net.IPNet) bool {
	if firstIP, _ := networkRange(netX); netY.Contains(firstIP) {
		return true
	}
	if firstIP, _ := networkRange(netY); netX.Contains(firstIP) {
		return true
	}
	return false
}

func networkRange(network *net.IPNet) (net.IP, net.IP) {
	var netIP net.IP
	if network.IP.To4() != nil {
		netIP = network.IP.To4()
	} else if network.IP.To16() != nil {
		netIP = network.IP.To16()
	} else {
		return nil, nil
	}

	lastIP := make([]byte, len(netIP), len(netIP))
	for i := 0; i < len(netIP); i++ {
		lastIP[i] = netIP[i] | ^network.Mask[i]
	}
	return netIP.Mask(network.Mask), net.IP(lastIP)
}
