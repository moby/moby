//go:build linux

package macvlan

import (
	"context"
	"fmt"
	"net"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/netlabel"
	"github.com/moby/moby/v2/daemon/libnetwork/netutils"
	"github.com/moby/moby/v2/daemon/libnetwork/ns"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Join method is invoked when a Sandbox is attached to an endpoint.
func (d *driver) Join(ctx context.Context, nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, epOpts, _ map[string]any) error {
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.drivers.macvlan.Join", trace.WithAttributes(
		attribute.String("nid", nid),
		attribute.String("eid", eid),
		attribute.String("sboxKey", sboxKey)))
	defer span.End()

	n, err := d.getNetwork(nid)
	if err != nil {
		return err
	}
	// generate a name for the iface that will be renamed to eth0 in the sbox
	containerIfName, err := netutils.GenerateIfaceName(ns.NlHandle(), vethPrefix, vethLen)
	if err != nil {
		return fmt.Errorf("error generating an interface name: %w", err)
	}
	// create the netlink macvlan interface
	vethName, err := createMacVlan(containerIfName, n.config.Parent, n.config.MacvlanMode)
	if err != nil {
		return err
	}
	ep := n.endpoint(eid)
	if ep == nil {
		return fmt.Errorf("could not find endpoint with id %s", eid)
	}
	// bind the generated iface name to the endpoint
	//
	// TODO(thaJeztah): this should really be done under a lock.
	ep.srcName = vethName

	if !n.config.Internal {
		// parse and correlate the endpoint v4 address with the available v4 subnets
		if ep.addr != nil && len(n.config.Ipv4Subnets) > 0 {
			s := n.getSubnetforIPv4(ep.addr)
			if s == nil {
				return fmt.Errorf("could not find a valid ipv4 subnet for endpoint %s", eid)
			}
			if s.GwIP == "" {
				// Can't set up a default gateway, but the network is not internal, so it should
				// be treated as having external connectivity. (This preserves old behavior,
				// where a gateway address was assigned from IPAM that did not necessarily
				// correspond with a working gateway.)
				jinfo.ForceGw4()
			} else {
				v4gw, _, err := net.ParseCIDR(s.GwIP)
				if err != nil {
					return fmt.Errorf("gateway %s is not a valid ipv4 address: %v", s.GwIP, err)
				}
				err = jinfo.SetGateway(v4gw)
				if err != nil {
					return err
				}
			}
			log.G(ctx).Debugf("Macvlan Endpoint Joined with IPv4_Addr: %s, Gateway: %s, MacVlan_Mode: %s, Parent: %s",
				ep.addr.IP.String(), s.GwIP, n.config.MacvlanMode, n.config.Parent)
		}
		// parse and correlate the endpoint v6 address with the available v6 subnets
		if ep.addrv6 != nil && len(n.config.Ipv6Subnets) > 0 {
			s := n.getSubnetforIPv6(ep.addrv6)
			if s == nil {
				return fmt.Errorf("could not find a valid ipv6 subnet for endpoint %s", eid)
			}
			if s.GwIP == "" {
				// Can't set up a default gateway, but the network is not internal, so it should
				// be treated as having external connectivity. (This preserves old behavior,
				// where a gateway address was assigned from IPAM that did not necessarily
				// correspond with a working gateway.)
				jinfo.ForceGw6()
			} else {
				v6gw, _, err := net.ParseCIDR(s.GwIP)
				if err != nil {
					return fmt.Errorf("gateway %s is not a valid ipv6 address: %v", s.GwIP, err)
				}
				err = jinfo.SetGatewayIPv6(v6gw)
				if err != nil {
					return err
				}
			}
			log.G(ctx).Debugf("Macvlan Endpoint Joined with IPv6_Addr: %s Gateway: %s MacVlan_Mode: %s, Parent: %s",
				ep.addrv6.IP.String(), s.GwIP, n.config.MacvlanMode, n.config.Parent)
		}
	} else {
		if len(n.config.Ipv4Subnets) > 0 {
			log.G(ctx).Debugf("Macvlan Endpoint Joined with IPv4_Addr: %s, MacVlan_Mode: %s, Parent: %s",
				ep.addr.IP.String(), n.config.MacvlanMode, n.config.Parent)
		}
		if len(n.config.Ipv6Subnets) > 0 {
			log.G(ctx).Debugf("Macvlan Endpoint Joined with IPv6_Addr: %s MacVlan_Mode: %s, Parent: %s",
				ep.addrv6.IP.String(), n.config.MacvlanMode, n.config.Parent)
		}
	}
	jinfo.DisableGatewayService()
	iNames := jinfo.InterfaceName()
	err = iNames.SetNames(vethName, containerVethPrefix, netlabel.GetIfname(epOpts))
	if err != nil {
		return err
	}
	if err := d.storeUpdate(ep); err != nil {
		return fmt.Errorf("failed to save macvlan endpoint %.7s to store: %v", ep.id, err)
	}

	return nil
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *driver) Leave(nid, eid string) error {
	network, err := d.getNetwork(nid)
	if err != nil {
		return err
	}
	endpoint, err := network.getEndpoint(eid)
	if err != nil {
		return err
	}
	if endpoint == nil {
		return fmt.Errorf("could not find endpoint with id %s", eid)
	}

	return nil
}

// getSubnetforIPv4 returns the ipv4 subnet to which the given IP belongs
func (n *network) getSubnetforIPv4(ip *net.IPNet) *ipSubnet {
	return getSubnetForIP(ip, n.config.Ipv4Subnets)
}

// getSubnetforIPv6 returns the ipv6 subnet to which the given IP belongs
func (n *network) getSubnetforIPv6(ip *net.IPNet) *ipSubnet {
	return getSubnetForIP(ip, n.config.Ipv6Subnets)
}

func getSubnetForIP(ip *net.IPNet, subnets []*ipSubnet) *ipSubnet {
	for _, s := range subnets {
		_, snet, err := net.ParseCIDR(s.SubnetIP)
		if err != nil {
			return nil
		}
		// first check if the mask lengths are the same
		i, _ := snet.Mask.Size()
		j, _ := ip.Mask.Size()
		if i != j {
			continue
		}
		if snet.Contains(ip.IP) {
			return s
		}
	}

	return nil
}
