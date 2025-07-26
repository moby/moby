package bridge

import (
	"errors"
	"fmt"
	"strings"

	"github.com/moby/moby/v2/daemon/libnetwork/ns"
	"github.com/vishvananda/netlink"
)

// setupVerifyAndReconcileIPv4 checks what IPv4 addresses the given i interface has
// and ensures that they match the passed network config.
func setupVerifyAndReconcileIPv4(config *networkConfiguration, i *bridgeInterface) error {
	if config.GwModeIPv4.isolated() {
		return nil
	}
	// Fetch a slice of IPv4 addresses from the bridge.
	addrsv4, err := i.addresses(netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("Failed to verify ip addresses: %v", err)
	}

	addrv4, _ := selectIPv4Address(addrsv4, config.AddressIPv4)

	// Verify that the bridge has an IPv4 address.
	if addrv4.IPNet == nil {
		return errors.New("bridge has no IPv4 address configured")
	}

	// Verify that the bridge IPv4 address matches the requested configuration.
	if config.AddressIPv4 != nil && !addrv4.IP.Equal(config.AddressIPv4.IP) {
		return fmt.Errorf("bridge IPv4 (%s) does not match requested configuration %s", addrv4.IP, config.AddressIPv4.IP)
	}

	return nil
}

func bridgeInterfaceExists(name string) (bool, error) {
	nlh := ns.NlHandle()
	link, err := nlh.LinkByName(name)
	if err != nil {
		if strings.Contains(err.Error(), "Link not found") {
			return false, nil
		}
		return false, fmt.Errorf("failed to check bridge interface existence: %v", err)
	}

	if link.Type() == "bridge" {
		return true, nil
	}
	return false, fmt.Errorf("existing interface %s is not a bridge", name)
}
