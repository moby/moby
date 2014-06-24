// +build linux

package network

import (
	"fmt"
)

// Loopback is a network strategy that provides a basic loopback device
type Loopback struct {
}

func (l *Loopback) Create(n *Network, nspid int, context map[string]string) error {
	return nil
}

func (l *Loopback) Initialize(config *Network, context map[string]string) error {
	if err := SetMtu("lo", config.Mtu); err != nil {
		return fmt.Errorf("set lo mtu to %d %s", config.Mtu, err)
	}
	if err := InterfaceUp("lo"); err != nil {
		return fmt.Errorf("lo up %s", err)
	}
	return nil
}
