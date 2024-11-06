package bridge

import (
	"fmt"
	"net/netip"
	"os"
)

// Standard link local prefix
var linkLocalPrefix = netip.MustParsePrefix("fe80::/64")

func setupBridgeIPv6(config *networkConfiguration, i *bridgeInterface) error {
	procFile := "/proc/sys/net/ipv6/conf/" + config.BridgeName + "/disable_ipv6"
	ipv6BridgeData, err := os.ReadFile(procFile)
	if err != nil {
		return fmt.Errorf("Cannot read IPv6 setup for bridge %v: %v", config.BridgeName, err)
	}
	// Enable IPv6 on the bridge only if it isn't already enabled
	if ipv6BridgeData[0] != '0' {
		if err := os.WriteFile(procFile, []byte{'0', '\n'}, 0o644); err != nil {
			return fmt.Errorf("Unable to enable IPv6 addresses on bridge: %v", err)
		}
	}

	// Remove unwanted addresses from the bridge, add required addresses, and assign
	// values to "i.bridgeIPv6", "i.gatewayIPv6".
	if err := i.programIPv6Addresses(config); err != nil {
		return err
	}
	return nil
}

func setupGatewayIPv6(config *networkConfiguration, i *bridgeInterface) error {
	if !config.AddressIPv6.Contains(config.DefaultGatewayIPv6) {
		return &ErrInvalidGateway{}
	}

	// Store requested default gateway
	i.gatewayIPv6 = config.DefaultGatewayIPv6

	return nil
}
