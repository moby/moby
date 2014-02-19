package network

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
)

// SetupVeth sets up an existing network namespace with the specified
// network configuration.
func SetupVeth(config *libcontainer.Network, tempVethName string) error {
	if err := InterfaceDown(tempVethName); err != nil {
		return fmt.Errorf("interface down %s %s", tempVethName, err)
	}
	if err := ChangeInterfaceName(tempVethName, "eth0"); err != nil {
		return fmt.Errorf("change %s to eth0 %s", tempVethName, err)
	}
	if err := SetInterfaceIp("eth0", config.IP); err != nil {
		return fmt.Errorf("set eth0 ip %s", err)
	}

	if err := SetMtu("eth0", config.Mtu); err != nil {
		return fmt.Errorf("set eth0 mtu to %d %s", config.Mtu, err)
	}
	if err := InterfaceUp("eth0"); err != nil {
		return fmt.Errorf("eth0 up %s", err)
	}

	if err := SetMtu("lo", config.Mtu); err != nil {
		return fmt.Errorf("set lo mtu to %d %s", config.Mtu, err)
	}
	if err := InterfaceUp("lo"); err != nil {
		return fmt.Errorf("lo up %s", err)
	}

	if config.Gateway != "" {
		if err := SetDefaultGateway(config.Gateway); err != nil {
			return fmt.Errorf("set gateway to %s %s", config.Gateway, err)
		}
	}
	return nil
}
