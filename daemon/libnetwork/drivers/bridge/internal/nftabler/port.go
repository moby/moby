//go:build linux

package nftabler

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge/internal/firewaller"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/nftables"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

type pbContext struct {
	table nftables.TableRef
	conf  firewaller.NetworkConfigFam
	ipv   firewaller.IPVersion
}

func (n *network) AddPorts(ctx context.Context, pbs []types.PortBinding) error {
	return n.modPorts(ctx, pbs, true)
}

func (n *network) DelPorts(ctx context.Context, pbs []types.PortBinding) error {
	return n.modPorts(ctx, pbs, false)
}

func (n *network) modPorts(ctx context.Context, pbs []types.PortBinding, enable bool) error {
	if n.config.Internal {
		return nil
	}

	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{"bridge": n.config.IfName}))

	if enable && n.fw.cleaner != nil {
		n.fw.cleaner.DelPorts(ctx, n.config, pbs)
	}

	pbs4, pbs6 := splitByContainerFam(pbs)
	if n.fw.config.IPv4 && n.config.Config4.Prefix.IsValid() {
		pbc := pbContext{table: n.fw.table4, conf: n.config.Config4, ipv: firewaller.IPv4}
		if err := n.setPerPortRules(ctx, pbs4, pbc, enable); err != nil {
			return err
		}
	}
	if n.fw.config.IPv6 && n.config.Config6.Prefix.IsValid() {
		pbc := pbContext{table: n.fw.table6, conf: n.config.Config6, ipv: firewaller.IPv6}
		if err := n.setPerPortRules(ctx, pbs6, pbc, enable); err != nil {
			return err
		}
	}
	return nil
}

func splitByContainerFam(pbs []types.PortBinding) ([]types.PortBinding, []types.PortBinding) {
	var pbs4, pbs6 []types.PortBinding
	for _, pb := range pbs {
		if pb.IP.To4() != nil {
			pbs4 = append(pbs4, pb)
		} else {
			pbs6 = append(pbs6, pb)
		}
	}
	return pbs4, pbs6
}

func (n *network) setPerPortRules(ctx context.Context, pbs []types.PortBinding, pbc pbContext, enable bool) error {
	if err := n.setPerPortForwarding(ctx, pbs, pbc, enable); err != nil {
		return err
	}
	if err := n.setPerPortDNAT(ctx, pbs, pbc, enable); err != nil {
		return err
	}
	if err := n.setPerPortHairpinMasq(ctx, pbs, pbc, enable); err != nil {
		return err
	}
	if err := n.filterPortMappedOnLoopback(ctx, pbs, pbc, enable); err != nil {
		return err
	}
	if err := nftApply(ctx, pbc.table); err != nil {
		return fmt.Errorf("adding rules for bridge %s: %w", n.config.IfName, err)
	}
	return nil
}

func (n *network) setPerPortForwarding(ctx context.Context, pbs []types.PortBinding, pbc pbContext, enable bool) error {
	if pbc.conf.Unprotected {
		return nil
	}
	updateFwdIn := pbc.table.ChainUpdateFunc(ctx, chainFilterFwdIn(n.config.IfName), enable)
	for _, pb := range pbs {
		// When more than one host port is mapped to a single container port, this will
		// generate the same rule for each host port. So, ignore duplicates when adding,
		// and missing rules when removing. (No ref-counting is currently needed because
		// when bindings are added or removed for an endpoint, they're all added or
		// removed for an address family. So, a rule that's added more than once will
		// also be deleted more than once.)
		//
		// TODO(robmry) - track port mappings, use that to edit nftables sets when bindings are added/removed.
		rule := fmt.Sprintf("%s daddr %s %s dport %d counter accept", pbc.table.Family(), pb.IP, pb.Proto, pb.Port)
		if err := updateFwdIn(ctx, fwdInPortsRuleGroup, rule); err != nil &&
			!errors.Is(err, nftables.ErrRuleExist) && !errors.Is(err, nftables.ErrRuleNotExist) {
			return fmt.Errorf("updating forwarding rule for port %s %s:%d/%s on %s, enable=%v: %w",
				pbc.table.Family(), pb.IP, pb.Port, pb.Proto, n.config.IfName, enable, err)
		}
	}
	return nil
}

func (n *network) setPerPortDNAT(ctx context.Context, pbs []types.PortBinding, pbc pbContext, enable bool) error {
	updater := pbc.table.ChainUpdateFunc(ctx, natChain, enable)
	var proxySkip string
	if !n.fw.config.Hairpin {
		proxySkip = fmt.Sprintf("iifname != %s ", n.config.IfName)
	}
	var v6LLSkip string
	if pbc.ipv == firewaller.IPv6 {
		v6LLSkip = "ip6 saddr != fe80::/10 "
	}
	for _, pb := range pbs {
		// Nothing to do if NAT is disabled.
		if pb.HostPort == 0 {
			continue
		}
		// If the binding is between containerV4 and hostV6, NAT isn't possible (the mapping
		// is handled by docker-proxy).
		if (pb.IP.To4() != nil) != (pb.HostIP.To4() != nil) {
			continue
		}
		var daddrMatch string
		if !pb.HostIP.IsUnspecified() {
			daddrMatch = fmt.Sprintf("%s daddr %s ", pbc.table.Family(), pb.HostIP)
		}
		rule := fmt.Sprintf("%s%s%s%s dport %d counter dnat to %s comment DNAT",
			proxySkip, v6LLSkip, daddrMatch, pb.Proto, pb.HostPort,
			net.JoinHostPort(pb.IP.String(), strconv.Itoa(int(pb.Port))))
		if err := updater(ctx, initialRuleGroup, rule); err != nil {
			return fmt.Errorf("adding DNAT for %s %s:%d -> %s:%d/%s on %s: %w",
				pbc.table.Family(), pb.HostIP, pb.HostPort, pb.IP, pb.Port, pb.Proto, n.config.IfName, err)
		}
	}
	return nil
}

// setPerPortHairpinMasq allows containers to access their own published ports on the host
// when hairpin is enabled (no docker-proxy), by masquerading.
func (n *network) setPerPortHairpinMasq(ctx context.Context, pbs []types.PortBinding, pbc pbContext, enable bool) error {
	if !n.fw.config.Hairpin {
		return nil
	}
	updater := pbc.table.ChainUpdateFunc(ctx, chainNatPostRtIn(n.config.IfName), enable)
	for _, pb := range pbs {
		// Nothing to do if NAT is disabled.
		if pb.HostPort == 0 {
			continue
		}
		// If the binding is between containerV4 and hostV6, NAT isn't possible (it's
		// handled by docker-proxy).
		if (pb.IP.To4() != nil) != (pb.HostIP.To4() != nil) {
			continue
		}
		// When more than one host port is mapped to a single container port, this will
		// generate the same rule for each host port. So, ignore duplicates when adding,
		// and missing rules when removing. (No ref-counting is currently needed because
		// when bindings are added or removed for an endpoint, they're all added or
		// removed. So, a rule that's added more than once will also be deleted more
		// than once.)
		//
		// TODO(robmry) - track port mappings, use that to edit nftables sets when bindings are added/removed.
		rule := fmt.Sprintf(`%s saddr %s %s daddr %s %s dport %d counter masquerade comment "MASQ TO OWN PORT"`,
			pbc.table.Family(), pb.IP, pbc.table.Family(), pb.IP, pb.Proto, pb.Port)
		if err := updater(ctx, initialRuleGroup, rule); err != nil &&
			!errors.Is(err, nftables.ErrRuleExist) && !errors.Is(err, nftables.ErrRuleNotExist) {
			return fmt.Errorf("adding MASQ TO OWN PORT for %d -> %s:%d/%s: %w",
				pb.Port, pb.IP, pb.Port, pb.Proto, err)
		}
	}
	return nil
}

// filterPortMappedOnLoopback adds a rule that drops remote connections to ports
// mapped to loopback addresses.
//
// This is a no-op if the portBinding is for IPv6 (IPv6 loopback address is
// non-routable), or over a network with gw_mode=routed (PBs in routed mode
// don't map ports on the host).
func (n *network) filterPortMappedOnLoopback(ctx context.Context, pbs []types.PortBinding, pbc pbContext, enable bool) error {
	if pbc.ipv == firewaller.IPv6 {
		return nil
	}
	updater := pbc.table.ChainUpdateFunc(ctx, rawPreroutingChain, enable)
	for _, pb := range pbs {
		// Nothing to do if not binding to the loopback address.
		if pb.HostPort == 0 || !pb.HostIP.IsLoopback() {
			continue
		}
		// Mappings from host IPv6 to container IPv4 are handled by docker-proxy.
		if pb.HostIP.To4() == nil {
			continue
		}
		if n.fw.config.WSL2Mirrored {
			if err := updater(ctx, rawPreroutingPortsRuleGroup,
				`iifname loopback0 ip daddr %s %s dport %d counter accept comment "%s"`,
				pb.HostIP, pb.Proto, pb.HostPort, "ACCEPT WSL2 LOOPBACK"); err != nil {
				return fmt.Errorf("adding WSL2 loopback rule for %d: %w", pb.HostPort, err)
			}
		}
		if err := updater(ctx, rawPreroutingPortsRuleGroup,
			`iifname != lo ip daddr %s %s dport %d counter drop comment "DROP REMOTE LOOPBACK"`,
			pb.HostIP, pb.Proto, pb.HostPort); err != nil {
			return fmt.Errorf("adding loopback filter rule for %d: %w", pb.HostPort, err)
		}
	}

	return nil
}
