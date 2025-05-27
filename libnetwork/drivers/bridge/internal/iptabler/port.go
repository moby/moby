//go:build linux

package iptabler

import (
	"context"
	"net"
	"os"
	"strconv"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/types"
)

func (n *network) AddPorts(ctx context.Context, pbs []types.PortBinding) error {
	return n.modPorts(ctx, pbs, true)
}

func (n *network) DelPorts(ctx context.Context, pbs []types.PortBinding) error {
	return n.modPorts(ctx, pbs, false)
}

func (n *network) modPorts(ctx context.Context, pbs []types.PortBinding, enable bool) error {
	for _, pb := range pbs {
		if err := n.setPerPortIptables(ctx, pb, enable); err != nil {
			return err
		}
	}
	return nil
}

// setPerPortIptables configures rules required by port binding b. Rules are added if
// enable is true, else removed.
func (n *network) setPerPortIptables(ctx context.Context, b types.PortBinding, enable bool) error {
	v := iptables.IPv4
	enabled := n.ipt.config.IPv4
	config := n.config.Config4
	if b.IP.To4() == nil {
		v = iptables.IPv6
		enabled = n.ipt.config.IPv6
		config = n.config.Config6
	}

	if !enabled || n.config.Internal {
		// Nothing to do.
		return nil
	}

	if err := filterPortMappedOnLoopback(ctx, b, b.HostIP, n.ipt.config.WSL2Mirrored, enable); err != nil {
		return err
	}

	if err := n.dropLegacyFilterDirectAccess(ctx, b); err != nil {
		return err
	}

	if (b.IP.To4() != nil) != (b.HostIP.To4() != nil) {
		// The binding is between containerV4 and hostV6 (not vice versa as that
		// will have been rejected earlier). It's handled by docker-proxy. So, no
		// further iptables rules are required.
		return nil
	}

	if err := n.setPerPortNAT(v, b, enable); err != nil {
		return err
	}

	if !config.Unprotected {
		if err := setPerPortForwarding(b, v, n.config.IfName, enable); err != nil {
			return err
		}
	}
	return nil
}

// setPerPortNAT configures DNAT and MASQUERADE rules for port binding b. Rules are added if
// enable is true, else removed.
func (n *network) setPerPortNAT(ipv iptables.IPVersion, b types.PortBinding, enable bool) error {
	if b.HostPort == 0 {
		// NAT is disabled.
		return nil
	}
	// iptables interprets "0.0.0.0" as "0.0.0.0/32", whereas we
	// want "0.0.0.0/0". "0/0" is correctly interpreted as "any
	// value" by both iptables and ip6tables.
	hostIP := "0/0"
	if !b.HostIP.IsUnspecified() {
		hostIP = b.HostIP.String()
	}
	args := []string{
		"-p", b.Proto.String(),
		"-d", hostIP,
		"--dport", strconv.Itoa(int(b.HostPort)),
		"-j", "DNAT",
		"--to-destination", net.JoinHostPort(b.IP.String(), strconv.Itoa(int(b.Port))),
	}
	if !n.ipt.config.Hairpin {
		args = append(args, "!", "-i", n.config.IfName)
	}
	if ipv == iptables.IPv6 {
		args = append(args, "!", "-s", "fe80::/10")
	}
	rule := iptables.Rule{IPVer: ipv, Table: iptables.Nat, Chain: dockerChain, Args: args}
	if err := appendOrDelChainRule(rule, "DNAT", enable); err != nil {
		return err
	}

	rule = iptables.Rule{IPVer: ipv, Table: iptables.Nat, Chain: "POSTROUTING", Args: []string{
		"-p", b.Proto.String(),
		"-s", b.IP.String(),
		"-d", b.IP.String(),
		"--dport", strconv.Itoa(int(b.Port)),
		"-j", "MASQUERADE",
	}}
	if err := appendOrDelChainRule(rule, "MASQUERADE", n.ipt.config.Hairpin && enable); err != nil {
		return err
	}

	return nil
}

// setPerPortForwarding opens access to a container's published port, as described by binding b.
// It also does something weird, broken, and disabled-by-default related to SCTP. Rules are added
// if enable is true, else removed.
func setPerPortForwarding(b types.PortBinding, ipv iptables.IPVersion, bridgeName string, enable bool) error {
	// Insert rules for open ports at the top of the filter table's DOCKER
	// chain (a per-network DROP rule, which must come after these per-port
	// per-container ACCEPT rules, is appended to the chain when the network
	// is created).
	rule := iptables.Rule{IPVer: ipv, Table: iptables.Filter, Chain: dockerChain, Args: []string{
		"!", "-i", bridgeName,
		"-o", bridgeName,
		"-p", b.Proto.String(),
		"-d", b.IP.String(),
		"--dport", strconv.Itoa(int(b.Port)),
		"-j", "ACCEPT",
	}}
	if err := programChainRule(rule, "OPEN PORT", enable); err != nil {
		return err
	}

	// TODO(robmry) - remove, see https://github.com/moby/moby/pull/48149
	if b.Proto == types.SCTP && os.Getenv("DOCKER_IPTABLES_SCTP_CHECKSUM") == "1" {
		// Linux kernel v4.9 and below enables NETIF_F_SCTP_CRC for veth by
		// the following commit.
		// This introduces a problem when combined with a physical NIC without
		// NETIF_F_SCTP_CRC. As for a workaround, here we add an iptables entry
		// to fill the checksum.
		//
		// https://github.com/torvalds/linux/commit/c80fafbbb59ef9924962f83aac85531039395b18
		rule := iptables.Rule{IPVer: ipv, Table: iptables.Mangle, Chain: "POSTROUTING", Args: []string{
			"-p", b.Proto.String(),
			"--sport", strconv.Itoa(int(b.Port)),
			"-j", "CHECKSUM",
			"--checksum-fill",
		}}
		if err := appendOrDelChainRule(rule, "SCTP CHECKSUM", enable); err != nil {
			return err
		}
	}

	return nil
}

// filterPortMappedOnLoopback adds an iptables rule that drops remote
// connections to ports mapped on loopback addresses.
//
// This is a no-op if the portBinding is for IPv6 (IPv6 loopback address is
// non-routable), or over a network with gw_mode=routed (PBs in routed mode
// don't map ports on the host).
func filterPortMappedOnLoopback(ctx context.Context, b types.PortBinding, hostIP net.IP, wsl2Mirrored, enable bool) error {
	if rawRulesDisabled(ctx) {
		return nil
	}
	if b.HostPort == 0 || !hostIP.IsLoopback() || hostIP.To4() == nil {
		return nil
	}

	acceptMirrored := iptables.Rule{IPVer: iptables.IPv4, Table: iptables.Raw, Chain: "PREROUTING", Args: []string{
		"-p", b.Proto.String(),
		"-d", hostIP.String(),
		"--dport", strconv.Itoa(int(b.HostPort)),
		"-i", "loopback0",
		"-j", "ACCEPT",
	}}
	enableMirrored := enable && wsl2Mirrored
	if err := appendOrDelChainRule(acceptMirrored, "LOOPBACK FILTERING - ACCEPT MIRRORED", enableMirrored); err != nil {
		return err
	}

	drop := iptables.Rule{IPVer: iptables.IPv4, Table: iptables.Raw, Chain: "PREROUTING", Args: []string{
		"-p", b.Proto.String(),
		"-d", hostIP.String(),
		"--dport", strconv.Itoa(int(b.HostPort)),
		"!", "-i", "lo",
		"-j", "DROP",
	}}
	if err := appendOrDelChainRule(drop, "LOOPBACK FILTERING - DROP", enable); err != nil {
		return err
	}

	return nil
}

// dropLegacyFilterDirectAccess deletes a rule that was introduced in 28.0.0 to
// drop 'direct' remote connections made to the container's IP address - for
// each published port on the container.
//
// The normal filter-FORWARD rules would then drop packets sent directly to
// unpublished ports. This rule was only created along with the rest of port
// publishing (when a container's endpoint was selected as its gateway). Until
// then, all packets addressed directly to the container's ports were dropped
// by the filter-FORWARD rules.
//
// Since 28.0.2, direct routed packets sent to a container's address are all
// dropped in a raw-PREROUTING rule - it doesn't need to be per-port (so, fewer
// rules), and it can be created along with the endpoint (so directly-routed
// packets are dropped at the same point whether or not the endpoint is currently
// the gateway - so, very slightly earlier when it's not the gateway).
//
// This function was a no-op if the gw_mode was "nat-unprotected" or "routed".
// It still is. but now always deletes the rule if it might have been created
// by an older version of the daemon.
//
// TODO(robmry) - remove this once there's no upgrade path from 28.0.x or 28.1.x.
func (n *network) dropLegacyFilterDirectAccess(ctx context.Context, b types.PortBinding) error {
	if rawRulesDisabled(ctx) {
		return nil
	}
	ipv := iptables.IPv4
	config := n.config.Config4
	if b.IP.To4() == nil {
		ipv = iptables.IPv6
		config = n.config.Config6
	}

	// gw_mode=nat-unprotected means there's minimal security for NATed ports,
	// so don't filter direct access.
	if config.Unprotected || config.Routed {
		return nil
	}

	drop := iptables.Rule{IPVer: ipv, Table: iptables.Raw, Chain: "PREROUTING", Args: []string{
		"-p", b.Proto.String(),
		"-d", b.IP.String(), // Container IP address
		"--dport", strconv.Itoa(int(b.Port)), // Container port
		"!", "-i", n.config.IfName,
		"-j", "DROP",
	}}
	if err := appendOrDelChainRule(drop, "LEGACY DIRECT ACCESS FILTERING - DROP", false); err != nil {
		return err
	}

	return nil
}

func rawRulesDisabled(ctx context.Context) bool {
	if os.Getenv("DOCKER_INSECURE_NO_IPTABLES_RAW") == "1" {
		log.G(ctx).Debug("DOCKER_INSECURE_NO_IPTABLES_RAW=1 - skipping raw rules")
		return true
	}
	return false
}
