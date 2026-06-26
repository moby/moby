package libnetwork

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/maputil"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/nftables"
)

func (n *Network) syncLBBackendsNftables(ctx context.Context, ep *Endpoint, sb *Sandbox, lb *loadBalancer, rmService bool) bool {
	// Add IP alias for the VIP to the endpoint
	ifName := findIfaceDstName(sb, ep)
	if ifName == "" {
		log.G(ctx).Errorf("Failed find interface name for endpoint %s(%s) to create LB alias", ep.ID(), ep.Name())
		return false
	}

	backends := maputil.FilterValues(lb.backEnds, func(backend *lbBackend) bool { return !backend.disabled })
	hasBackends := len(backends) > 0
	// newService/delService are the transitions of the VIP's programmed state,
	// tracked by lb.nftServiceProgrammed. Keying them on that tracked state rather
	// than the live backend count keeps the VIP alias and ingress ports added
	// exactly once and removed exactly once, even when the backend count
	// transiently drops to zero and recovers (e.g. during a rolling update).
	newService := !lb.nftServiceProgrammed && hasBackends
	delService := lb.nftServiceProgrammed && rmService && !hasBackends

	vip := lb.vip.String()
	if newService {
		log.G(ctx).Debugf("Creating service for vip %s ingressPorts %#v in sbox %.7s (%.7s)", vip, lb.service.ingressPorts, sb.ID(), sb.ContainerID())

		err := sb.osSbox.AddAliasIP(ifName, &net.IPNet{IP: lb.vip, Mask: net.CIDRMask(32, 32)})
		// The alias is added before the NAT map is programmed below. If a previous
		// sync added it but then failed to program the map, nftServiceProgrammed
		// stayed false, so this retry adds it again - tolerate it already being
		// present rather than aborting before the backends are programmed.
		if err != nil && !errors.Is(err, os.ErrExist) {
			log.G(ctx).Errorf("Failed add IP alias %s to network %s LB endpoint interface %s: %v", vip, n.ID(), ifName, err)
			return false
		}
	} else if delService {
		err := sb.osSbox.RemoveAliasIP(ifName, &net.IPNet{IP: lb.vip, Mask: net.CIDRMask(32, 32)})
		if err != nil {
			log.G(ctx).Errorf("Failed remove IP alias %s from network %s LB endpoint interface %s: %v", vip, n.ID(), ifName, err)
			return false
		}
	}

	const (
		natServiceVipMap  = "nat-service-vip"
		natPublishPortMap = "nat-publish-port"
		modulus           = 65536
		numgenExpr        = "numgen random mod 65536"
	)
	backendIntervals := nftables.EqualWeightIntervals(backends, modulus)
	var natAdd nftables.Modifier
	for i, b := range backendIntervals {
		bip := b.ip.String()
		natAdd.Create(nftables.MapElement{
			MapName: natServiceVipMap,
			Key:     fmt.Sprintf("%s . %s", vip, i),
			Value:   bip,
		})
		for _, p := range lb.service.ingressPorts {
			natAdd.Create(nftables.MapElement{
				MapName: natPublishPortMap,
				Key:     fmt.Sprintf("%s . %v . %s", strings.ToLower(p.Protocol.String()), p.PublishedPort, i),
				Value:   bip,
			})
		}
	}

	err := sb.osSbox.ApplyNFTable(ctx, nftables.IPv4, "docker-lb-nat", func(natInit *nftables.Modifier) error {
		natInit.Create(nftables.Map{
			Name:        natServiceVipMap,
			ElementType: nftables.Typeof("ip daddr").Concat(numgenExpr).MapTo("ip daddr"),
			Flags:       []string{"interval"},
			Counter:     true,
		})

		natInit.Create(nftables.Map{
			Name:        natPublishPortMap,
			ElementType: nftables.Typeof("meta l4proto . th dport").Concat(numgenExpr).MapTo("ip daddr"),
			Flags:       []string{"interval"},
			Counter:     true,
		})

		nftables.BaseChain{
			Name:      "prerouting",
			ChainType: nftables.BaseChainTypeNAT,
			Hook:      nftables.BaseChainHookPrerouting,
			Priority:  nftables.BaseChainPriorityDstNAT,
			Policy:    nftables.BaseChainPolicyAccept,
		}.Builder().
			Rule("dnat to ip daddr .", numgenExpr, "map @"+natServiceVipMap).
			Rule("dnat to meta l4proto . th dport .", numgenExpr, "map @"+natPublishPortMap).
			Create(natInit)

		nftables.BaseChain{
			Name:      "postrouting",
			ChainType: nftables.BaseChainTypeNAT,
			Hook:      nftables.BaseChainHookPostrouting,
			Priority:  nftables.BaseChainPrioritySrcNAT,
			Policy:    nftables.BaseChainPolicyAccept,
		}.Builder().
			Rule("ct status dnat counter masquerade").
			Create(natInit)
		return nil
	}, lb.nftClearNAT, natAdd)
	if err != nil {
		log.G(ctx).WithError(err).Error("Failed to apply changes to nftables nat table")
		// The backend NAT map wasn't programmed, so don't report the service as
		// newly created - otherwise the caller would publish its ingress ports for
		// a service whose NAT map is missing.
		newService = false
	} else {
		lb.nftClearNAT = natAdd.Reverse()
		if newService {
			lb.nftServiceProgrammed = true
		} else if delService {
			lb.nftServiceProgrammed = false
		}
	}

	if n.loadBalancerMode == loadBalancerModeDSR {
		const (
			dsrVipSet        = "vip"
			dsrRealServerMap = "real-server"
		)
		var dsrAdd nftables.Modifier
		// Only (re-)add the VIP to the set while the service has backends. On a
		// teardown sync (no backends), nftClearDSR removes it and it must stay
		// removed, otherwise the VIP would linger in the set forever.
		if len(backends) > 0 {
			dsrAdd.Create(nftables.SetElement{
				SetName: dsrVipSet,
				Element: vip,
				Comment: fmt.Sprintf("%s (%s)", lb.service.name, lb.service.id),
			})
		}

		for i, b := range backendIntervals {
			bip := b.ip.String()
			dsrAdd.Create(nftables.MapElement{
				MapName: dsrRealServerMap,
				Key:     fmt.Sprintf("%s . %s", vip, i),
				Value:   bip,
			})
		}

		err := sb.osSbox.ApplyNFTable(ctx, nftables.Netdev, "docker-lb-dsr", func(dsrInit *nftables.Modifier) error {
			l4protos := []string{"tcp", "udp", "sctp"}

			dsrInit.Create(nftables.Set{
				Name:        dsrVipSet,
				ElementType: nftables.IPv4Addr,
				Counter:     true,
			})
			dsrInit.Create(nftables.Map{
				Name:        dsrRealServerMap,
				ElementType: nftables.Typeof("ip daddr").Concat(numgenExpr).MapTo("ip daddr"),
				Flags:       []string{"interval"},
				Counter:     true,
			})

			// Sticky overlay DSR sessions.
			// Split by L4 protocol as nftables 1.0.6 crashes when attempting to
			// compile a ruleset that contains a map update on a 5-tuple key.
			for _, l4proto := range l4protos {
				dsrInit.Create(nftables.Map{
					Name:        "dsr-conntrack-" + l4proto,
					ElementType: nftables.Typeof("ip saddr . th sport . ip daddr . th dport").MapTo("ether daddr"),
					Flags:       []string{"dynamic"},
					Size:        65536,
					Timeout:     60 * time.Second,
				})
			}

			b := nftables.BaseChain{
				Name:      "ingress",
				ChainType: nftables.BaseChainTypeFilter,
				Hook:      nftables.BaseChainHookIngress,
				Device:    ifName,
				Priority:  nftables.BaseChainPriorityFilter,
				Policy:    nftables.BaseChainPolicyAccept,
			}.Builder().
				Rule("meta protocol arp counter accept").
				Rule("ip daddr != @"+dsrVipSet, "counter accept").
				Rule("notrack ether saddr set ether daddr counter")

			for _, l4proto := range l4protos {
				// Established session: reuse MAC from conntrack map.
				b.Rule("ip protocol", l4proto,
					"ether daddr set ip saddr . th sport . ip daddr . th dport",
					"map @dsr-conntrack-"+l4proto, "counter fwd to", ifName)
			}
			b.
				// New session: random bucket lookup. The session is
				// persisted to the conntrack map in the egress chain.
				Rule("ip protocol {", strings.Join(l4protos, ", "), "}",
					"fwd ip to ip daddr .", numgenExpr, "map @"+dsrRealServerMap, "device", ifName).
				// The service is defined but the packet does not correspond to
				// an established session and no backends are available for a new
				// session.
				Rule("counter drop").
				Create(dsrInit)

			b = nftables.BaseChain{
				Name:      "egress",
				ChainType: nftables.BaseChainTypeFilter,
				Hook:      nftables.BaseChainHookEgress,
				Device:    ifName,
				Priority:  nftables.BaseChainPriorityFilter,
				Policy:    nftables.BaseChainPolicyAccept,
			}.Builder().
				Rule("meta protocol arp counter accept").
				Rule("ip daddr != @"+dsrVipSet, "counter accept").
				// We can confidently stop tracking a TCP session that has been reset.
				Rule("tcp flags rst update @dsr-conntrack-tcp",
					"{ ip saddr . th sport . ip daddr . th dport : ether daddr timeout 0s }",
					"counter accept")
			for _, l4proto := range l4protos {
				b.Rule("ip protocol", l4proto, "update @dsr-conntrack-"+l4proto,
					"{ ip saddr . th sport . ip daddr . th dport : ether daddr }",
					"counter accept")
			}
			b.Create(dsrInit)
			return nil
		}, lb.nftClearDSR, dsrAdd)

		if err != nil {
			log.G(ctx).WithError(err).Error("Failed to apply changes to nftables dsr table")
		} else {
			lb.nftClearDSR = dsrAdd.Reverse()
		}
	}

	return newService
}

func (sb *Sandbox) addRedirectRulesNftables(ctx context.Context, eIP *net.IPNet, ingressPorts []*PortConfig) error {
	if sb.osSbox == nil {
		return nil
	}

	// The iptables implementation skips programming the rules to drop
	// packets addressed to eIP on unpublished ports when the list of
	// ingress ports is empty. It's a very strange behavior, but it's been
	// like that for a decade, so we are replicating it here for
	// consistency.
	if len(ingressPorts) == 0 {
		return nil
	}

	const (
		publishedPortsMap = "published-ports"
		ingressIPsSet     = "ingress-ips"
	)
	var tm nftables.Modifier
	ingressIP := eIP.IP.String()
	tm.Create(nftables.SetElement{
		SetName: ingressIPsSet,
		Element: ingressIP,
	})
	for _, p := range ingressPorts {
		tm.Create(nftables.MapElement{
			MapName: publishedPortsMap,
			Key:     fmt.Sprintf("%s . %s . %d", ingressIP, strings.ToLower(p.Protocol.String()), p.PublishedPort),
			Value:   strconv.FormatUint(uint64(p.TargetPort), 10),
			Comment: p.Name,
		})
	}

	err := sb.osSbox.ApplyNFTable(ctx, nftables.IPv4, "docker-container-ingress", func(initIngress *nftables.Modifier) error {
		// Map of ingress-IP . publishPort -> targetPort.
		// Packets with a destination address of ingress-IP and a
		// destination port in this map are redirected to the target
		// port.
		initIngress.Create(nftables.Map{
			Name:        publishedPortsMap,
			ElementType: nftables.IPv4Addr.Concat(nftables.InetProto).Concat(nftables.InetService).MapTo(nftables.InetService),
		})
		initIngress.Create(nftables.Set{
			Name:        ingressIPsSet,
			ElementType: nftables.IPv4Addr,
		})

		nftables.BaseChain{
			Name:      "prerouting",
			ChainType: nftables.BaseChainTypeNAT,
			Hook:      nftables.BaseChainHookPrerouting,
			Priority:  nftables.BaseChainPriorityDstNAT,
			Policy:    nftables.BaseChainPolicyAccept,
		}.Builder().
			Rule("meta l4proto { tcp, udp, sctp } redirect to ip daddr . meta l4proto . th dport map @" + publishedPortsMap).
			Create(initIngress)

		nftables.BaseChain{
			Name:      "input",
			ChainType: nftables.BaseChainTypeFilter,
			Hook:      nftables.BaseChainHookInput,
			Priority:  nftables.BaseChainPriorityFilter,
			Policy:    nftables.BaseChainPolicyAccept,
		}.Builder().
			Rule("ip daddr != @"+ingressIPsSet, "counter accept").
			Rule("icmp type { destination-unreachable, time-exceeded } counter accept").
			// Only allow incoming connections to exposed ports
			// from the ingress network.
			Rule("ct state { established, related } counter accept").
			Rule("ct status dnat counter accept").
			Rule("counter reject").
			Create(initIngress)

		nftables.BaseChain{
			Name:      "output",
			ChainType: nftables.BaseChainTypeFilter,
			Hook:      nftables.BaseChainHookOutput,
			Priority:  nftables.BaseChainPriorityFilter,
			Policy:    nftables.BaseChainPolicyAccept,
		}.Builder().
			Rule("ip saddr != @"+ingressIPsSet, "counter accept").
			Rule("icmp type { destination-unreachable, time-exceeded } counter accept").
			// Only allow outgoing replies for incoming connections
			// to be transmitted over the ingress network.
			Rule("ct status dnat ct state { established, related } counter accept").
			Rule("counter reject").
			Create(initIngress)

		return nil
	}, tm)
	if err != nil {
		return fmt.Errorf("failed to add redirect rules to nftables ingress table: %v", err)
	}
	return nil
}
