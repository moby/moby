package bridge

import (
	"fmt"
	"io/ioutil"
	"net"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/netutils"
	"github.com/vishvananda/netlink"
)

var bridgeNetworks []*net.IPNet

func init() {
	// Here we don't follow the convention of using the 1st IP of the range for the gateway.
	// This is to use the same gateway IPs as the /24 ranges, which predate the /16 ranges.
	// In theory this shouldn't matter - in practice there's bound to be a few scripts relying
	// on the internal addressing or other stupid things like that.
	// They shouldn't, but hey, let's not break them unless we really have to.
	// Don't use 172.16.0.0/16, it conflicts with EC2 DNS 172.16.0.23

	// 172.[17-31].42.1/16
	mask := []byte{255, 255, 0, 0}
	for i := 17; i < 32; i++ {
		bridgeNetworks = append(bridgeNetworks, &net.IPNet{IP: []byte{172, byte(i), 42, 1}, Mask: mask})
	}
	// 10.[0-255].42.1/16
	for i := 0; i < 256; i++ {
		bridgeNetworks = append(bridgeNetworks, &net.IPNet{IP: []byte{10, byte(i), 42, 1}, Mask: mask})
	}
	// 192.168.[42-44].1/24
	mask[2] = 255
	for i := 42; i < 45; i++ {
		bridgeNetworks = append(bridgeNetworks, &net.IPNet{IP: []byte{192, 168, byte(i), 1}, Mask: mask})
	}
}

func setupBridgeIPv4(config *networkConfiguration, i *bridgeInterface) error {
	addrv4, _, err := i.addresses()
	if err != nil {
		return err
	}

	// Check if we have an IP address already on the bridge.
	if addrv4.IPNet != nil {
		// Make sure to store bridge network and default gateway before getting out.
		i.bridgeIPv4 = addrv4.IPNet
		i.gatewayIPv4 = addrv4.IPNet.IP
		return nil
	}

	// Do not try to configure IPv4 on a non-default bridge unless you are
	// specifically asked to do so.
	if config.BridgeName != DefaultBridgeName && !config.AllowNonDefaultBridge {
		return NonDefaultBridgeExistError(config.BridgeName)
	}

	bridgeIPv4, err := electBridgeIPv4(config)
	if err != nil {
		return err
	}

	log.Debugf("Creating bridge interface %q with network %s", config.BridgeName, bridgeIPv4)
	if err := netlink.AddrAdd(i.Link, &netlink.Addr{IPNet: bridgeIPv4}); err != nil {
		return &IPv4AddrAddError{IP: bridgeIPv4, Err: err}
	}

	// Store bridge network and default gateway
	i.bridgeIPv4 = bridgeIPv4
	i.gatewayIPv4 = i.bridgeIPv4.IP

	return nil
}

func allocateBridgeIP(config *networkConfiguration, i *bridgeInterface) error {
	ipAllocator.RequestIP(i.bridgeIPv4, i.bridgeIPv4.IP)
	return nil
}

func electBridgeIPv4(config *networkConfiguration) (*net.IPNet, error) {
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

	// Try to automatically elect appropriate bridge IPv4 settings.
	for _, n := range bridgeNetworks {
		if err := netutils.CheckNameserverOverlaps(nameservers, n); err == nil {
			if err := netutils.CheckRouteOverlaps(n); err == nil {
				return n, nil
			}
		}
	}

	return nil, IPv4AddrRangeError(config.BridgeName)
}

func setupGatewayIPv4(config *networkConfiguration, i *bridgeInterface) error {
	if !i.bridgeIPv4.Contains(config.DefaultGatewayIPv4) {
		return &ErrInvalidGateway{}
	}
	if _, err := ipAllocator.RequestIP(i.bridgeIPv4, config.DefaultGatewayIPv4); err != nil {
		return err
	}

	// Store requested default gateway
	i.gatewayIPv4 = config.DefaultGatewayIPv4

	return nil
}

func setupLoopbackAdressesRouting(config *networkConfiguration, i *bridgeInterface) error {
	// Enable loopback adresses routing
	sysPath := filepath.Join("/proc/sys/net/ipv4/conf", config.BridgeName, "route_localnet")
	if err := ioutil.WriteFile(sysPath, []byte{'1', '\n'}, 0644); err != nil {
		return fmt.Errorf("Unable to enable local routing for hairpin mode: %v", err)
	}
	return nil
}
