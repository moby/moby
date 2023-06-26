//go:build linux

package bridge

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/containerd/containerd/log"
	"github.com/vishvananda/netlink"
)

// bridgeIPv6 is the default, link-local IPv6 address for the bridge (fe80::1/64)
var bridgeIPv6 = &net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)}

const (
	ipv6ForwardConfPerm    = 0644
	ipv6ForwardConfDefault = "/proc/sys/net/ipv6/conf/default/forwarding"
	ipv6ForwardConfAll     = "/proc/sys/net/ipv6/conf/all/forwarding"
)

func setupBridgeIPv6(config *networkConfiguration, i *bridgeInterface) error {
	procFile := "/proc/sys/net/ipv6/conf/" + config.BridgeName + "/disable_ipv6"
	ipv6BridgeData, err := os.ReadFile(procFile)
	if err != nil {
		return fmt.Errorf("Cannot read IPv6 setup for bridge %v: %v", config.BridgeName, err)
	}
	// Enable IPv6 on the bridge only if it isn't already enabled
	if ipv6BridgeData[0] != '0' {
		if err := os.WriteFile(procFile, []byte{'0', '\n'}, ipv6ForwardConfPerm); err != nil {
			return fmt.Errorf("Unable to enable IPv6 addresses on bridge: %v", err)
		}
	}

	// Store bridge network and default gateway
	i.bridgeIPv6 = bridgeIPv6
	i.gatewayIPv6 = i.bridgeIPv6.IP

	if err := i.programIPv6Address(); err != nil {
		return err
	}

	if config.AddressIPv6 == nil {
		return nil
	}

	// Store the user specified bridge network and network gateway and program it
	i.bridgeIPv6 = config.AddressIPv6
	i.gatewayIPv6 = config.AddressIPv6.IP

	if err := i.programIPv6Address(); err != nil {
		return err
	}

	// Setting route to global IPv6 subnet
	log.G(context.TODO()).Debugf("Adding route to IPv6 network %s via device %s", config.AddressIPv6.String(), config.BridgeName)
	err = i.nlh.RouteAdd(&netlink.Route{
		Scope:     netlink.SCOPE_UNIVERSE,
		LinkIndex: i.Link.Attrs().Index,
		Dst:       config.AddressIPv6,
	})
	if err != nil && !os.IsExist(err) {
		log.G(context.TODO()).Errorf("Could not add route to IPv6 network %s via device %s: %s", config.AddressIPv6.String(), config.BridgeName, err)
	}

	return nil
}

func setupGatewayIPv6(config *networkConfiguration, i *bridgeInterface) error {
	if config.AddressIPv6 == nil {
		return &ErrInvalidContainerSubnet{}
	}
	if !config.AddressIPv6.Contains(config.DefaultGatewayIPv6) {
		return &ErrInvalidGateway{}
	}

	// Store requested default gateway
	i.gatewayIPv6 = config.DefaultGatewayIPv6

	return nil
}

func setupIPv6Forwarding(config *networkConfiguration, i *bridgeInterface) error {
	// Get current IPv6 default forwarding setup
	ipv6ForwardDataDefault, err := os.ReadFile(ipv6ForwardConfDefault)
	if err != nil {
		return fmt.Errorf("Cannot read IPv6 default forwarding setup: %v", err)
	}
	// Enable IPv6 default forwarding only if it is not already enabled
	if ipv6ForwardDataDefault[0] != '1' {
		if err := os.WriteFile(ipv6ForwardConfDefault, []byte{'1', '\n'}, ipv6ForwardConfPerm); err != nil {
			log.G(context.TODO()).Warnf("Unable to enable IPv6 default forwarding: %v", err)
		}
	}

	// Get current IPv6 all forwarding setup
	ipv6ForwardDataAll, err := os.ReadFile(ipv6ForwardConfAll)
	if err != nil {
		return fmt.Errorf("Cannot read IPv6 all forwarding setup: %v", err)
	}
	// Enable IPv6 all forwarding only if it is not already enabled
	if ipv6ForwardDataAll[0] != '1' {
		if err := os.WriteFile(ipv6ForwardConfAll, []byte{'1', '\n'}, ipv6ForwardConfPerm); err != nil {
			log.G(context.TODO()).Warnf("Unable to enable IPv6 all forwarding: %v", err)
		}
	}

	return nil
}
