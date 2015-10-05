package bridge

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
)

func setupBridgeIPv4(config *networkConfiguration, i *bridgeInterface) error {
	addrv4, _, err := i.addresses()
	if err != nil {
		return fmt.Errorf("failed to retrieve bridge interface addresses: %v", err)
	}

	if !types.CompareIPNet(addrv4.IPNet, config.AddressIPv4) {
		if addrv4.IPNet != nil {
			if err := netlink.AddrDel(i.Link, &addrv4); err != nil {
				return fmt.Errorf("failed to remove current ip address from bridge: %v", err)
			}
		}
		log.Debugf("Assigning address to bridge interface %s: %s", config.BridgeName, config.AddressIPv4)
		if err := netlink.AddrAdd(i.Link, &netlink.Addr{IPNet: config.AddressIPv4}); err != nil {
			return &IPv4AddrAddError{IP: config.AddressIPv4, Err: err}
		}
	}

	// Store bridge network and default gateway
	i.bridgeIPv4 = config.AddressIPv4
	i.gatewayIPv4 = config.AddressIPv4.IP

	return nil
}

func setupGatewayIPv4(config *networkConfiguration, i *bridgeInterface) error {
	if !i.bridgeIPv4.Contains(config.DefaultGatewayIPv4) {
		return &ErrInvalidGateway{}
	}

	// Store requested default gateway
	i.gatewayIPv4 = config.DefaultGatewayIPv4

	return nil
}

func setupLoopbackAdressesRouting(config *networkConfiguration, i *bridgeInterface) error {
	sysPath := filepath.Join("/proc/sys/net/ipv4/conf", config.BridgeName, "route_localnet")
	ipv4LoRoutingData, err := ioutil.ReadFile(sysPath)
	if err != nil {
		return fmt.Errorf("Cannot read IPv4 local routing setup: %v", err)
	}
	// Enable loopback adresses routing only if it isn't already enabled
	if ipv4LoRoutingData[0] != '1' {
		if err := ioutil.WriteFile(sysPath, []byte{'1', '\n'}, 0644); err != nil {
			return fmt.Errorf("Unable to enable local routing for hairpin mode: %v", err)
		}
	}
	return nil
}
