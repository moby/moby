//go:build linux

package bridge

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/nlwrap"
	"github.com/vishvananda/netlink"
)

// noBrNfConfig describes how a network routes traffic between its containers
// through the host, instead of relying on br_netfilter to send bridged packets
// through the IP stack's netfilter hooks.
//
// Without br_netfilter, packets bridged between two containers on the same network
// aren't seen by iptables/nftables rules or conntrack. So, the ICC drop rule has no
// effect, and when the userland proxy is disabled, replies to DNATed packets from a
// container that addressed another container on the same network via a published
// port are bridged straight back to the client without being un-DNATed.
//
// Instead:
//   - Container addresses are configured without on-link prefix routes, with the
//     prefix routed via the bridge's address. So, containers send unicast IP for
//     other containers on the network to the host, which routes it. That means the
//     existing ICC rules and conntrack see it.
//   - When ICC is disabled, container ports on the bridge are isolated, so
//     containers can't reach each other at layer 2 (no broadcast, multicast,
//     non-IP protocols, or bypassing the routing by adding an on-link route).
type noBrNfConfig struct {
	// isolatePorts is true if container ports must be isolated (icc=false).
	isolatePorts bool
	// gw4 and gw6 are the addresses containers route their network's prefix
	// through. If not valid, containers have the usual on-link route for that
	// address family.
	gw4 netip.Addr
	gw6 netip.Addr
}

// setupNoBrNetfilter is a bridge setup step, used in place of the br_netfilter
// setup steps when the driver is configured with NoBrNetfilter. It works out
// how the network will route traffic between containers through the host.
//
// Routing through the host is only possible for an address family if the host
// has an address on the bridge, that address is the gateway, and forwarding is
// enabled (the same condition as for using br_netfilter).
func (n *bridgeNetwork) setupNoBrNetfilter(config *networkConfiguration, i *bridgeInterface) error {
	d := n.driver
	var nc noBrNfConfig
	nc.isolatePorts = !config.EnableICC

	if d.config.EnableIPTables && config.EnableIPv4 && !config.InhibitIPv4 &&
		!config.GwModeIPv4.isolated() && config.DefaultGatewayIPv4 == nil && i.bridgeIPv4 != nil {
		if enabled, err := getKernelBoolParam("/proc/sys/net/ipv4/ip_forward"); err != nil {
			return fmt.Errorf("checking IPv4 forwarding: %w", err)
		} else if enabled {
			gw, ok := netip.AddrFromSlice(i.bridgeIPv4.IP)
			if !ok {
				return fmt.Errorf("invalid bridge address %s", i.bridgeIPv4)
			}
			nc.gw4 = gw.Unmap()
		}
	}

	if d.config.EnableIP6Tables && config.EnableIPv6 &&
		!config.GwModeIPv6.isolated() && config.DefaultGatewayIPv6 == nil && i.bridgeIPv6 != nil {
		if enabled, err := getKernelBoolParam("/proc/sys/net/ipv6/conf/" + config.BridgeName + "/forwarding"); err != nil {
			return fmt.Errorf("checking IPv6 forwarding: %w", err)
		} else if enabled {
			gw, ok := netip.AddrFromSlice(i.bridgeIPv6.IP)
			if !ok {
				return fmt.Errorf("invalid bridge address %s", i.bridgeIPv6)
			}
			nc.gw6 = gw
		}
	}

	log.G(context.TODO()).WithFields(log.Fields{
		"bridge":       config.BridgeName,
		"isolatePorts": nc.isolatePorts,
		"gw4":          nc.gw4,
		"gw6":          nc.gw6,
	}).Debug("Routing traffic between containers via the host, not using br_netfilter")

	n.Lock()
	n.noBrNf = nc
	n.Unlock()
	return nil
}

// setupEndpointNoBrNetfilter configures a new endpoint for routing through the
// host, if the network needs it. host is the host side of the endpoint's veth,
// which must already be attached to the bridge.
func (n *bridgeNetwork) setupEndpointNoBrNetfilter(nlh nlwrap.Handle, host netlink.Link, ifInfo driverapi.InterfaceInfo) error {
	n.Lock()
	nc := n.noBrNf
	n.Unlock()

	if nc.isolatePorts {
		if err := nlh.LinkSetIsolated(host, true); err != nil {
			return fmt.Errorf("setting bridge port %s isolated: %w", host.Attrs().Name, err)
		}
	}
	if nc.gw4.IsValid() || nc.gw6.IsValid() {
		ifInfo.SetPrefixGateways(nc.gw4, nc.gw6)
	}
	return nil
}
