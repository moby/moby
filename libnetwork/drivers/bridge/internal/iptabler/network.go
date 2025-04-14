//go:build linux

package iptabler

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/containerd/log"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libnetwork/iptables"
)

type (
	iptableCleanFunc   func() error
	iptablesCleanFuncs []iptableCleanFunc
)

type NetworkConfigFam struct {
	HostIP      netip.Addr
	Prefix      netip.Prefix
	Routed      bool
	Unprotected bool
}

type NetworkConfig struct {
	IfName                string
	Internal              bool
	ICC                   bool
	Masquerade            bool
	TrustedHostInterfaces []string
	Config4               NetworkConfigFam
	Config6               NetworkConfigFam
}

type Network struct {
	NetworkConfig
	ipt        *Iptabler
	cleanFuncs iptablesCleanFuncs
}

func (ipt *Iptabler) NewNetwork(nc NetworkConfig) (_ *Network, retErr error) {
	n := &Network{
		ipt:           ipt,
		NetworkConfig: nc,
	}
	defer func() {
		if retErr != nil {
			if err := n.DelNetworkLevelRules(); err != nil {
				log.G(context.TODO()).WithError(err).Warnf("Failed to delete network level rules following earlier error")
			}
		}
	}()

	if err := n.ReapplyNetworkLevelRules(); err != nil {
		return nil, err
	}
	return n, nil
}

func (n *Network) ReapplyNetworkLevelRules() error {
	if n.ipt.IPv4 {
		if err := n.configure(iptables.IPv4, n.Config4); err != nil {
			return err
		}
	}
	if n.ipt.IPv6 {
		if err := n.configure(iptables.IPv6, n.Config6); err != nil {
			return err
		}
	}
	return nil
}

func (n *Network) DelNetworkLevelRules() error {
	var errs []error
	for _, cleanFunc := range n.cleanFuncs {
		if err := cleanFunc(); err != nil {
			errs = append(errs, err)
		}
	}
	n.cleanFuncs = nil
	return errors.Join(errs...)
}

func (n *Network) configure(ipv iptables.IPVersion, conf NetworkConfigFam) error {
	if !conf.Prefix.IsValid() {
		// Delete INC rules, in case they were created by a 28.0.0 daemon that didn't check
		// whether the network had iptables/ip6tables enabled.
		// This preserves https://github.com/moby/moby/commit/8cc4d1d4a2b6408232041f9ba4dff966eba80cc0
		return setINC(ipv, n.IfName, conf.Routed, false)
	}
	if err := n.setupIPTables(ipv, conf); err != nil {
		return err
	}
	return nil
}

func (n *Network) registerCleanFunc(clean iptableCleanFunc) {
	n.cleanFuncs = append(n.cleanFuncs, clean)
}

func (n *Network) setupIPTables(ipVersion iptables.IPVersion, config NetworkConfigFam) error {
	if n.Internal {
		if err := setupInternalNetworkRules(n.IfName, config.Prefix, n.ICC, true); err != nil {
			return fmt.Errorf("Failed to Setup IP tables: %w", err)
		}
		n.registerCleanFunc(func() error {
			return setupInternalNetworkRules(n.IfName, config.Prefix, n.ICC, false)
		})
	} else {
		if err := n.setupNonInternalNetworkRules(ipVersion, config, true); err != nil {
			return fmt.Errorf("Failed to Setup IP tables: %w", err)
		}
		n.registerCleanFunc(func() error {
			return n.setupNonInternalNetworkRules(ipVersion, config, false)
		})

		if err := iptables.AddInterfaceFirewalld(n.IfName); err != nil {
			return err
		}
		n.registerCleanFunc(func() error {
			if err := iptables.DelInterfaceFirewalld(n.IfName); err != nil && !errdefs.IsNotFound(err) {
				return err
			}
			return nil
		})

		if err := deleteLegacyFilterRules(ipVersion, n.IfName); err != nil {
			return fmt.Errorf("failed to delete legacy rules in filter-FORWARD: %w", err)
		}

		err := setDefaultForwardRule(ipVersion, n.IfName, config.Unprotected, true)
		if err != nil {
			return err
		}
		n.registerCleanFunc(func() error {
			return setDefaultForwardRule(ipVersion, n.IfName, config.Unprotected, false)
		})

		ctRule := iptables.Rule{IPVer: ipVersion, Table: iptables.Filter, Chain: dockerCTChain, Args: []string{
			"-o", n.IfName,
			"-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED",
			"-j", "ACCEPT",
		}}
		if err := appendOrDelChainRule(ctRule, "bridge ct related", true); err != nil {
			return err
		}
		n.registerCleanFunc(func() error {
			return appendOrDelChainRule(ctRule, "bridge ct related", false)
		})
		jumpToDockerRule := iptables.Rule{IPVer: ipVersion, Table: iptables.Filter, Chain: dockerBridgeChain, Args: []string{
			"-o", n.IfName,
			"-j", dockerChain,
		}}
		if err := appendOrDelChainRule(jumpToDockerRule, "jump to docker", true); err != nil {
			return err
		}
		n.registerCleanFunc(func() error {
			return appendOrDelChainRule(jumpToDockerRule, "jump to docker", false)
		})

		// Register the cleanup function first. Then, if setINC fails after creating
		// some rules, they will be deleted.
		n.registerCleanFunc(func() error {
			return setINC(ipVersion, n.IfName, config.Routed, false)
		})
		if err := setINC(ipVersion, n.IfName, config.Routed, true); err != nil {
			return err
		}
	}
	return nil
}

func setICMP(ipv iptables.IPVersion, bridgeName string, enable bool) error {
	icmpProto := "icmp"
	if ipv == iptables.IPv6 {
		icmpProto = "icmpv6"
	}
	icmpRule := iptables.Rule{IPVer: ipv, Table: iptables.Filter, Chain: dockerChain, Args: []string{
		"-o", bridgeName,
		"-p", icmpProto,
		"-j", "ACCEPT",
	}}
	return appendOrDelChainRule(icmpRule, "ICMP", enable)
}

func addNATJumpRules(ipVer iptables.IPVersion, hairpinMode, enable bool) error {
	preroute := iptables.Rule{IPVer: ipVer, Table: iptables.Nat, Chain: "PREROUTING", Args: []string{
		"-m", "addrtype",
		"--dst-type", "LOCAL",
		"-j", dockerChain,
	}}
	if enable {
		if err := preroute.Append(); err != nil {
			return fmt.Errorf("failed to append jump rules to nat-PREROUTING: %s", err)
		}
	} else {
		if err := preroute.Delete(); err != nil {
			return fmt.Errorf("failed to remove jump rules from nat-PREROUTING: %s", err)
		}
	}

	output := iptables.Rule{IPVer: ipVer, Table: iptables.Nat, Chain: "OUTPUT", Args: []string{
		"-m", "addrtype",
		"--dst-type", "LOCAL",
		"-j", dockerChain,
	}}
	if !hairpinMode {
		output.Args = append(output.Args, "!", "--dst", loopbackAddress(ipVer))
	}
	if enable {
		if err := output.Append(); err != nil {
			return fmt.Errorf("failed to append jump rules to nat-OUTPUT: %s", err)
		}
	} else {
		if err := output.Delete(); err != nil {
			return fmt.Errorf("failed to remove jump rules from nat-OUTPUT: %s", err)
		}
	}

	return nil
}

// deleteLegacyFilterRules removes the legacy per-bridge rules from the filter-FORWARD
// chain. This is required for users upgrading the Engine to v28.0.
// TODO(aker): drop this function once Mirantis latest LTS is v28.0 (or higher).
func deleteLegacyFilterRules(ipVer iptables.IPVersion, bridgeName string) error {
	iptable := iptables.GetIptable(ipVer)
	// Delete legacy per-bridge jump to the DOCKER chain from the FORWARD chain, if it exists.
	// These rules have been replaced by an ipset-matching rule.
	link := []string{
		"-o", bridgeName,
		"-j", dockerChain,
	}
	if iptable.Exists(iptables.Filter, "FORWARD", link...) {
		del := append([]string{string(iptables.Delete), "FORWARD"}, link...)
		if output, err := iptable.Raw(del...); err != nil {
			return err
		} else if len(output) != 0 {
			return fmt.Errorf("could not delete linking rule from %s-%s: %s", iptables.Filter, dockerChain, output)
		}
	}

	// Delete legacy per-bridge related/established rule if it exists. These rules
	// have been replaced by an ipset-matching rule.
	establish := []string{
		"-o", bridgeName,
		"-m", "conntrack",
		"--ctstate", "RELATED,ESTABLISHED",
		"-j", "ACCEPT",
	}
	if iptable.Exists(iptables.Filter, "FORWARD", establish...) {
		del := append([]string{string(iptables.Delete), "FORWARD"}, establish...)
		if output, err := iptable.Raw(del...); err != nil {
			return err
		} else if len(output) != 0 {
			return fmt.Errorf("could not delete establish rule from %s-%s: %s", iptables.Filter, dockerChain, output)
		}
	}

	return nil
}

// loopbackAddress returns the loopback address for the given IP version.
func loopbackAddress(version iptables.IPVersion) string {
	switch version {
	case iptables.IPv4, "":
		// IPv4 (default for backward-compatibility)
		return "127.0.0.0/8"
	case iptables.IPv6:
		return "::1/128"
	default:
		panic("unknown IP version: " + version)
	}
}

func setDefaultForwardRule(ipVersion iptables.IPVersion, ifName string, unprotected bool, enable bool) error {
	// Normally, DROP anything that hasn't been ACCEPTed by a per-port/protocol
	// rule. This prevents direct access to un-mapped ports from remote hosts
	// that can route directly to the container's address (by setting up a
	// route via the host's address).
	action := "DROP"
	if unprotected {
		// If the user really wants to allow all access from the wider network,
		// explicitly ACCEPT anything so that the filter-FORWARD chain's
		// default policy can't interfere.
		action = "ACCEPT"
	}

	rule := iptables.Rule{IPVer: ipVersion, Table: iptables.Filter, Chain: dockerChain, Args: []string{
		"!", "-i", ifName,
		"-o", ifName,
		"-j", action,
	}}

	// Append to the filter table's DOCKER chain (the default rule must follow
	// per-port ACCEPT rules, which will be inserted at the top of the chain).
	if err := appendOrDelChainRule(rule, "DEFAULT FWD", enable); err != nil {
		return fmt.Errorf("failed to add default-drop rule: %w", err)
	}
	return nil
}

func (n *Network) setupNonInternalNetworkRules(ipVer iptables.IPVersion, config NetworkConfigFam, enable bool) error {
	var natArgs, hpNatArgs []string
	if config.HostIP.IsValid() {
		// The user wants IPv4/IPv6 SNAT with the given address.
		hostAddr := config.HostIP.String()
		natArgs = []string{"-s", config.Prefix.String(), "!", "-o", n.IfName, "-j", "SNAT", "--to-source", hostAddr}
		hpNatArgs = []string{"-m", "addrtype", "--src-type", "LOCAL", "-o", n.IfName, "-j", "SNAT", "--to-source", hostAddr}
	} else {
		// Use MASQUERADE, which picks the src-ip based on next-hop from the route table
		natArgs = []string{"-s", config.Prefix.String(), "!", "-o", n.IfName, "-j", "MASQUERADE"}
		hpNatArgs = []string{"-m", "addrtype", "--src-type", "LOCAL", "-o", n.IfName, "-j", "MASQUERADE"}
	}
	natRule := iptables.Rule{IPVer: ipVer, Table: iptables.Nat, Chain: "POSTROUTING", Args: natArgs}
	hpNatRule := iptables.Rule{IPVer: ipVer, Table: iptables.Nat, Chain: "POSTROUTING", Args: hpNatArgs}

	// Set NAT.
	nat := !config.Routed
	if n.Masquerade {
		if nat {
			if err := programChainRule(natRule, "NAT", enable); err != nil {
				return err
			}
		}
		// If the userland proxy is running (!hairpin), skip DNAT for packets originating from
		// this new network. Then, the proxy can pick up the packet from the host address the dest
		// port is published to. Otherwise, if the packet is DNAT'd, it's forwarded straight to the
		// target network, and will be dropped by network isolation rules if it didn't originate in
		// the same bridge network. (So, with the proxy enabled, this skip allows a container in one
		// network to reach a port published by a container in another bridge network.)
		//
		// If the userland proxy is disabled, don't skip, so packets will be DNAT'd. That will
		// enable access to ports published by containers in the same network. But, the INC rules
		// will block access to that published port from containers in other networks. (However,
		// users may add a rule to DOCKER-USER to work around the INC rules if needed.)
		if !n.ipt.Hairpin {
			skipDNAT := iptables.Rule{IPVer: ipVer, Table: iptables.Nat, Chain: dockerChain, Args: []string{
				"-i", n.IfName,
				"-j", "RETURN",
			}}
			if err := programChainRule(skipDNAT, "SKIP DNAT", enable); err != nil {
				return err
			}
		}
	}

	// In hairpin mode, masquerade traffic from localhost. If hairpin is disabled or if we're tearing down
	// that bridge, make sure the iptables rule isn't lying around.
	if err := programChainRule(hpNatRule, "MASQ LOCAL HOST", enable && n.ipt.Hairpin); err != nil {
		return err
	}

	// Set Inter Container Communication.
	if err := setIcc(ipVer, n.IfName, n.ICC, false, enable); err != nil {
		return err
	}

	// Allow ICMP in routed mode.
	if !nat {
		if err := setICMP(ipVer, n.IfName, enable); err != nil {
			return err
		}
	}

	// Handle outgoing packets. This rule was previously added unconditionally
	// to ACCEPT packets that weren't ICC - an extra rule was needed to enable
	// ICC if needed. Those rules are now combined. So, outRuleNoICC is only
	// needed for ICC=false, along with the DROP rule for ICC added by setIcc.
	outRuleNoICC := iptables.Rule{IPVer: ipVer, Table: iptables.Filter, Chain: DockerForwardChain, Args: []string{
		"-i", n.IfName,
		"!", "-o", n.IfName,
		"-j", "ACCEPT",
	}}
	// If there's a version of outRuleNoICC in the FORWARD chain, created by moby 28.0.0 or older, delete it.
	if enable {
		if err := outRuleNoICC.WithChain("FORWARD").Delete(); err != nil {
			return fmt.Errorf("deleting FORWARD chain outRuleNoICC: %w", err)
		}
	}
	if n.ICC {
		// Accept outgoing traffic to anywhere, including other containers on this bridge.
		outRuleICC := iptables.Rule{IPVer: ipVer, Table: iptables.Filter, Chain: DockerForwardChain, Args: []string{
			"-i", n.IfName,
			"-j", "ACCEPT",
		}}
		if err := appendOrDelChainRule(outRuleICC, "ACCEPT OUTGOING", enable); err != nil {
			return err
		}
		// If there's a version of outRuleICC in the FORWARD chain, created by moby 28.0.0 or older, delete it.
		if enable {
			if err := outRuleICC.WithChain("FORWARD").Delete(); err != nil {
				return fmt.Errorf("deleting FORWARD chain outRuleICC: %w", err)
			}
		}
	} else {
		// Accept outgoing traffic to anywhere, apart from other containers on this bridge.
		// setIcc added a DROP rule for ICC traffic.
		if err := appendOrDelChainRule(outRuleNoICC, "ACCEPT NON_ICC OUTGOING", enable); err != nil {
			return err
		}
	}

	return nil
}

func setIcc(version iptables.IPVersion, bridgeIface string, iccEnable, internal, insert bool) error {
	args := []string{"-i", bridgeIface, "-o", bridgeIface, "-j"}
	acceptRule := iptables.Rule{IPVer: version, Table: iptables.Filter, Chain: DockerForwardChain, Args: append(args, "ACCEPT")}
	dropRule := iptables.Rule{IPVer: version, Table: iptables.Filter, Chain: DockerForwardChain, Args: append(args, "DROP")}

	// The accept rule is no longer required for a bridge with external connectivity, because
	// ICC traffic is allowed by the outgoing-packets rule created by setupIptablesInternal.
	// The accept rule is still required for a --internal network because it has no outgoing
	// rule. If insert and the rule is not required, an ACCEPT rule for an external network
	// may have been left behind by an older version of the daemon so, delete it.
	if insert && iccEnable && internal {
		if err := acceptRule.Append(); err != nil {
			return fmt.Errorf("Unable to allow intercontainer communication: %w", err)
		}
	} else {
		if err := acceptRule.Delete(); err != nil {
			log.G(context.TODO()).WithError(err).Warn("Failed to delete legacy ICC accept rule")
		}
	}

	if insert && !iccEnable {
		if err := dropRule.Append(); err != nil {
			return fmt.Errorf("Unable to prevent intercontainer communication: %w", err)
		}
	} else {
		if err := dropRule.Delete(); err != nil {
			log.G(context.TODO()).WithError(err).Warn("Failed to delete ICC drop rule")
		}
	}

	// Delete rules that may have been inserted into the FORWARD chain by moby 28.0.0 or older.
	if insert {
		if err := acceptRule.WithChain("FORWARD").Delete(); err != nil {
			return fmt.Errorf("deleting FORWARD chain accept rule: %w", err)
		}
		if err := dropRule.WithChain("FORWARD").Delete(); err != nil {
			return fmt.Errorf("deleting FORWARD chain drop rule: %w", err)
		}
	}
	return nil
}

// Control Inter-Network Communication.
// Install rules only if they aren't present, remove only if they are.
// If this method returns an error, it doesn't roll back any rules it has added.
// No error is returned if rules cannot be removed (errors are just logged).
func setINC(version iptables.IPVersion, iface string, routed, enable bool) (retErr error) {
	iptable := iptables.GetIptable(version)
	actionI, actionA := iptables.Insert, iptables.Append
	actionMsg := "add"
	if !enable {
		actionI, actionA = iptables.Delete, iptables.Delete
		actionMsg = "remove"
	}

	if routed {
		// Anything is allowed into a routed network at this stage, so RETURN. Port
		// filtering rules in the DOCKER chain will drop anything that's not destined
		// for an open port.
		if err := iptable.ProgramRule(iptables.Filter, isolationChain1, actionI, []string{
			"-o", iface,
			"-j", "RETURN",
		}); err != nil {
			log.G(context.TODO()).WithError(err).Warnf("Failed to %s inter-network communication rule", actionMsg)
			if enable {
				return fmt.Errorf("%s inter-network communication rule: %w", actionMsg, err)
			}
		}

		// Allow responses from the routed network into whichever network made the request.
		if err := iptable.ProgramRule(iptables.Filter, isolationChain1, actionI, []string{
			"-i", iface,
			"-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED",
			"-j", "ACCEPT",
		}); err != nil {
			log.G(context.TODO()).WithError(err).Warnf("Failed to %s inter-network communication rule", actionMsg)
			if enable {
				return fmt.Errorf("%s inter-network communication rule: %w", actionMsg, err)
			}
		}
	}

	if err := iptable.ProgramRule(iptables.Filter, isolationChain1, actionA, []string{
		"-i", iface,
		"!", "-o", iface,
		"-j", isolationChain2,
	}); err != nil {
		log.G(context.TODO()).WithError(err).Warnf("Failed to %s inter-network communication rule", actionMsg)
		if enable {
			return fmt.Errorf("%s inter-network communication rule: %w", actionMsg, err)
		}
	}

	if err := iptable.ProgramRule(iptables.Filter, isolationChain2, actionI, []string{
		"-o", iface,
		"-j", "DROP",
	}); err != nil {
		log.G(context.TODO()).WithError(err).Warnf("Failed to %s inter-network communication rule", actionMsg)
		if enable {
			return fmt.Errorf("%s inter-network communication rule: %w", actionMsg, err)
		}
	}

	return nil
}

// Obsolete chain from previous docker versions
const oldIsolationChain = "DOCKER-ISOLATION"

func removeIPChains(version iptables.IPVersion) {
	ipt := iptables.GetIptable(version)

	// Remove obsolete rules from default chains
	ipt.ProgramRule(iptables.Filter, "FORWARD", iptables.Delete, []string{"-j", oldIsolationChain})

	// Remove chains
	for _, chainInfo := range []iptables.ChainInfo{
		{Name: dockerChain, Table: iptables.Nat, IPVersion: version},
		{Name: dockerChain, Table: iptables.Filter, IPVersion: version},
		{Name: DockerForwardChain, Table: iptables.Filter, IPVersion: version},
		{Name: dockerBridgeChain, Table: iptables.Filter, IPVersion: version},
		{Name: dockerCTChain, Table: iptables.Filter, IPVersion: version},
		{Name: isolationChain1, Table: iptables.Filter, IPVersion: version},
		{Name: isolationChain2, Table: iptables.Filter, IPVersion: version},
		{Name: oldIsolationChain, Table: iptables.Filter, IPVersion: version},
	} {
		if err := chainInfo.Remove(); err != nil {
			log.G(context.TODO()).Warnf("Failed to remove existing iptables entries in table %s chain %s : %v", chainInfo.Table, chainInfo.Name, err)
		}
	}
}

func setupInternalNetworkRules(bridgeIface string, prefix netip.Prefix, icc, insert bool) error {
	var version iptables.IPVersion
	var inDropRule, outDropRule iptables.Rule

	// Either add or remove the interface from the firewalld zone, if firewalld is running.
	if insert {
		if err := iptables.AddInterfaceFirewalld(bridgeIface); err != nil {
			return err
		}
	} else {
		if err := iptables.DelInterfaceFirewalld(bridgeIface); err != nil && !errdefs.IsNotFound(err) {
			return err
		}
	}

	if prefix.Addr().Is4() {
		version = iptables.IPv4
		inDropRule = iptables.Rule{
			IPVer: version,
			Table: iptables.Filter,
			Chain: isolationChain1,
			Args:  []string{"-i", bridgeIface, "!", "-d", prefix.String(), "-j", "DROP"},
		}
		outDropRule = iptables.Rule{
			IPVer: version,
			Table: iptables.Filter,
			Chain: isolationChain1,
			Args:  []string{"-o", bridgeIface, "!", "-s", prefix.String(), "-j", "DROP"},
		}
	} else {
		version = iptables.IPv6
		inDropRule = iptables.Rule{
			IPVer: version,
			Table: iptables.Filter,
			Chain: isolationChain1,
			Args:  []string{"-i", bridgeIface, "!", "-o", bridgeIface, "!", "-d", prefix.String(), "-j", "DROP"},
		}
		outDropRule = iptables.Rule{
			IPVer: version,
			Table: iptables.Filter,
			Chain: isolationChain1,
			Args:  []string{"!", "-i", bridgeIface, "-o", bridgeIface, "!", "-s", prefix.String(), "-j", "DROP"},
		}
	}

	if err := programChainRule(inDropRule, "DROP INCOMING", insert); err != nil {
		return err
	}
	if err := programChainRule(outDropRule, "DROP OUTGOING", insert); err != nil {
		return err
	}

	// Set Inter Container Communication.
	return setIcc(version, bridgeIface, icc, true, insert)
}
