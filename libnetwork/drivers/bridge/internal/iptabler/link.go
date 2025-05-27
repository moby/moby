//go:build linux

package iptabler

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/types"
)

func (n *network) AddLink(ctx context.Context, parentIP, childIP netip.Addr, ports []types.TransportPort) error {
	if !parentIP.IsValid() || parentIP.IsUnspecified() {
		return fmt.Errorf("cannot link to a container with an empty parent IP address")
	}
	if !childIP.IsValid() || childIP.IsUnspecified() {
		return fmt.Errorf("cannot link to a container with an empty child IP address")
	}

	chain := iptables.ChainInfo{Name: dockerChain}
	for _, port := range ports {
		if err := chain.Link(iptables.Append, parentIP, childIP, int(port.Port), port.Proto.String(), n.config.IfName); err != nil {
			return err
		}
	}
	return nil
}

func (n *network) DelLink(ctx context.Context, parentIP, childIP netip.Addr, ports []types.TransportPort) {
	chain := iptables.ChainInfo{Name: dockerChain}
	for _, port := range ports {
		if err := chain.Link(iptables.Delete, parentIP, childIP, int(port.Port), port.Proto.String(), n.config.IfName); err != nil {
			log.G(ctx).WithFields(log.Fields{
				"parentIP": parentIP,
				"childIP":  childIP,
				"port":     port.Port,
				"protocol": port.Proto.String(),
				"bridge":   n.config.IfName,
			}).Warn("Failed to remove link between containers")
		}
	}
}
