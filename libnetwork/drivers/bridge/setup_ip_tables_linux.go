package bridge

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/containerd/log"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/internal/nlwrap"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
)

// DockerChain: DOCKER iptable chain name
const (
	DockerChain = "DOCKER"

	// Isolation between bridge networks is achieved in two stages by means
	// of the following two chains in the filter table. The first chain matches
	// on the source interface being a bridge network's bridge and the
	// destination being a different interface. A positive match leads to the
	// second isolation chain. No match returns to the parent chain. The second
	// isolation chain matches on destination interface being a bridge network's
	// bridge. A positive match identifies a packet originated from one bridge
	// network's bridge destined to another bridge network's bridge and will
	// result in the packet being dropped. No match returns to the parent chain.

	IsolationChain1 = "DOCKER-ISOLATION-STAGE-1"
	IsolationChain2 = "DOCKER-ISOLATION-STAGE-2"

	// ipset names for IPv4 and IPv6 bridge subnets that don't belong
	// to --internal networks.
	ipsetExtBridges4 = "docker-ext-bridges-v4"
	ipsetExtBridges6 = "docker-ext-bridges-v6"
)

// Path to the executable installed in Linux under WSL2 that reports on
// WSL config. https://github.com/microsoft/WSL/releases/tag/2.0.4
// Can be modified by tests.
var wslinfoPath = "/usr/bin/wslinfo"

func setupIPChains(config configuration, version iptables.IPVersion) (natChain *iptables.ChainInfo, filterChain *iptables.ChainInfo, isolationChain1 *iptables.ChainInfo, isolationChain2 *iptables.ChainInfo, retErr error) {
	// Sanity check.
	if version == iptables.IPv4 && !config.EnableIPTables {
		return nil, nil, nil, nil, errors.New("cannot create new chains, iptables is disabled")
	}
	if version == iptables.IPv6 && !config.EnableIP6Tables {
		return nil, nil, nil, nil, errors.New("cannot create new chains, ip6tables is disabled")
	}

	iptable := iptables.GetIptable(version)

	natChain, err := iptable.NewChain(DockerChain, iptables.Nat)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create NAT chain %s: %v", DockerChain, err)
	}
	defer func() {
		if retErr != nil {
			if err := iptable.RemoveExistingChain(DockerChain, iptables.Nat); err != nil {
				log.G(context.TODO()).Warnf("failed on removing iptables NAT chain %s on cleanup: %v", DockerChain, err)
			}
		}
	}()

	filterChain, err = iptable.NewChain(DockerChain, iptables.Filter)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create FILTER chain %s: %v", DockerChain, err)
	}
	defer func() {
		if err != nil {
			if err := iptable.RemoveExistingChain(DockerChain, iptables.Filter); err != nil {
				log.G(context.TODO()).Warnf("failed on removing iptables FILTER chain %s on cleanup: %v", DockerChain, err)
			}
		}
	}()

	isolationChain1, err = iptable.NewChain(IsolationChain1, iptables.Filter)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create FILTER isolation chain: %v", err)
	}
	defer func() {
		if retErr != nil {
			if err := iptable.RemoveExistingChain(IsolationChain1, iptables.Filter); err != nil {
				log.G(context.TODO()).Warnf("failed on removing iptables FILTER chain %s on cleanup: %v", IsolationChain1, err)
			}
		}
	}()

	isolationChain2, err = iptable.NewChain(IsolationChain2, iptables.Filter)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create FILTER isolation chain: %v", err)
	}
	defer func() {
		if retErr != nil {
			if err := iptable.RemoveExistingChain(IsolationChain2, iptables.Filter); err != nil {
				log.G(context.TODO()).Warnf("failed on removing iptables FILTER chain %s on cleanup: %v", IsolationChain2, err)
			}
		}
	}()

	if err := mirroredWSL2Workaround(config, version); err != nil {
		return nil, nil, nil, nil, err
	}

	return natChain, filterChain, isolationChain1, isolationChain2, nil
}

func (n *bridgeNetwork) setupIP4Tables(config *networkConfiguration, i *bridgeInterface) error {
	d := n.driver
	d.Lock()
	driverConfig := d.config
	d.Unlock()

	// Sanity check.
	if !driverConfig.EnableIPTables {
		return errors.New("Cannot program chains, EnableIPTable is disabled")
	}

	maskedAddrv4 := &net.IPNet{
		IP:   i.bridgeIPv4.IP.Mask(i.bridgeIPv4.Mask),
		Mask: i.bridgeIPv4.Mask,
	}
	return n.setupIPTables(iptables.IPv4, maskedAddrv4, config, i)
}

func (n *bridgeNetwork) setupIP6Tables(config *networkConfiguration, i *bridgeInterface) error {
	d := n.driver
	d.Lock()
	driverConfig := d.config
	d.Unlock()

	// Sanity check.
	if !driverConfig.EnableIP6Tables {
		return errors.New("Cannot program chains, EnableIP6Tables is disabled")
	}

	maskedAddrv6 := &net.IPNet{
		IP:   i.bridgeIPv6.IP.Mask(i.bridgeIPv6.Mask),
		Mask: i.bridgeIPv6.Mask,
	}

	return n.setupIPTables(iptables.IPv6, maskedAddrv6, config, i)
}

func (n *bridgeNetwork) setupIPTables(ipVersion iptables.IPVersion, maskedAddr *net.IPNet, config *networkConfiguration, i *bridgeInterface) error {
	var err error

	d := n.driver
	d.Lock()
	driverConfig := d.config
	d.Unlock()

	// Pickup this configuration option from driver
	hairpinMode := !driverConfig.EnableUserlandProxy

	iptable := iptables.GetIptable(ipVersion)
	ipsetName := ipsetExtBridges4
	if ipVersion == iptables.IPv6 {
		ipsetName = ipsetExtBridges6
	}

	if config.Internal {
		if err = setupInternalNetworkRules(config.BridgeName, maskedAddr, config.EnableICC, true); err != nil {
			return fmt.Errorf("Failed to Setup IP tables: %s", err.Error())
		}
		n.registerIptCleanFunc(func() error {
			return setupInternalNetworkRules(config.BridgeName, maskedAddr, config.EnableICC, false)
		})
	} else {
		if err = setupIPTablesInternal(ipVersion, config, maskedAddr, hairpinMode, true); err != nil {
			return fmt.Errorf("Failed to Setup IP tables: %s", err.Error())
		}
		n.registerIptCleanFunc(func() error {
			return setupIPTablesInternal(ipVersion, config, maskedAddr, hairpinMode, false)
		})

		natChain, filterChain, _, _, err := n.getDriverChains(ipVersion)
		if err != nil {
			return fmt.Errorf("Failed to setup IP tables, cannot acquire chain info %s", err.Error())
		}

		err = iptable.ProgramChain(natChain, config.BridgeName, hairpinMode, true)
		if err != nil {
			return fmt.Errorf("Failed to program NAT chain: %s", err.Error())
		}

		err = iptable.ProgramChain(filterChain, config.BridgeName, hairpinMode, true)
		if err != nil {
			return fmt.Errorf("Failed to program FILTER chain: %s", err.Error())
		}
		n.registerIptCleanFunc(func() error {
			return iptable.ProgramChain(filterChain, config.BridgeName, hairpinMode, false)
		})

		if err := defaultDrop(ipVersion, config.BridgeName, true); err != nil {
			return fmt.Errorf("failed to add default-drop rule: %s", err.Error())
		}
		n.registerIptCleanFunc(func() error {
			return defaultDrop(ipVersion, config.BridgeName, false)
		})

		cidr, _ := maskedAddr.Mask.Size()
		if cidr == 0 {
			return fmt.Errorf("no CIDR for bridge %s addr %s", config.BridgeName, maskedAddr)
		}
		ipsetEntry := &netlink.IPSetEntry{
			IP:   maskedAddr.IP,
			CIDR: uint8(cidr),
		}
		if err := netlink.IpsetAdd(ipsetName, ipsetEntry); err != nil {
			return fmt.Errorf("failed to add bridge %s (%s) to ipset: %w",
				config.BridgeName, maskedAddr, err)
		}
		n.registerIptCleanFunc(func() error {
			return netlink.IpsetDel(ipsetName, ipsetEntry)
		})
	}

	d.Lock()
	defer d.Unlock()
	if err := iptable.EnsureJumpRule("FORWARD", DockerChain,
		"-m", "set", "--match-set", ipsetName, "dst"); err != nil {
		return err
	}
	if err := iptable.EnsureJumpRule("FORWARD", IsolationChain1); err != nil {
		return err
	}
	if err := iptable.EnsureJumpRule("FORWARD", "ACCEPT",
		"-m", "set", "--match-set", ipsetName, "dst",
		"-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED",
	); err != nil {
		return err
	}
	return nil
}

func setICMP(ipv iptables.IPVersion, bridgeName string, enable bool) error {
	icmpProto := "icmp"
	if ipv == iptables.IPv6 {
		icmpProto = "icmpv6"
	}
	icmpRule := iptRule{ipv: ipv, table: iptables.Filter, chain: DockerChain, args: []string{
		"-o", bridgeName,
		"-p", icmpProto,
		"-j", "ACCEPT",
	}}
	return appendOrDelChainRule(icmpRule, "ICMP", enable)
}

// Append to the filter table's DOCKER chain (the default DROP rule must follow
// per-port ACCEPT rules, which will be inserted at the top of the chain).
func defaultDrop(ipv iptables.IPVersion, bridgeName string, enable bool) error {
	dropRule := iptRule{ipv: ipv, table: iptables.Filter, chain: DockerChain, args: []string{
		"!", "-i", bridgeName,
		"-o", bridgeName,
		"-j", "DROP",
	}}
	return appendOrDelChainRule(dropRule, "DEFAULT DROP", enable)
}

type iptRule struct {
	ipv   iptables.IPVersion
	table iptables.Table
	chain string
	args  []string
}

// Exists returns true if the rule exists in the kernel.
func (r iptRule) Exists() bool {
	return iptables.GetIptable(r.ipv).Exists(r.table, r.chain, r.args...)
}

func (r iptRule) cmdArgs(op iptables.Action) []string {
	return append([]string{"-t", string(r.table), string(op), r.chain}, r.args...)
}

func (r iptRule) exec(op iptables.Action) error {
	return iptables.GetIptable(r.ipv).RawCombinedOutput(r.cmdArgs(op)...)
}

// Append appends the rule to the end of the chain. If the rule already exists anywhere in the
// chain, this is a no-op.
func (r iptRule) Append() error {
	if r.Exists() {
		return nil
	}
	return r.exec(iptables.Append)
}

// Insert inserts the rule at the head of the chain. If the rule already exists anywhere in the
// chain, this is a no-op.
func (r iptRule) Insert() error {
	if r.Exists() {
		return nil
	}
	return r.exec(iptables.Insert)
}

// Delete deletes the rule from the kernel. If the rule does not exist, this is a no-op.
func (r iptRule) Delete() error {
	if !r.Exists() {
		return nil
	}
	return r.exec(iptables.Delete)
}

func (r iptRule) String() string {
	cmd := append([]string{"iptables"}, r.cmdArgs("-A")...)
	if r.ipv == iptables.IPv6 {
		cmd[0] = "ip6tables"
	}
	return strings.Join(cmd, " ")
}

func setupIPTablesInternal(ipVer iptables.IPVersion, config *networkConfiguration, addr *net.IPNet, hairpin, enable bool) error {
	var (
		address   = addr.String()
		skipDNAT  = iptRule{ipv: ipVer, table: iptables.Nat, chain: DockerChain, args: []string{"-i", config.BridgeName, "-j", "RETURN"}}
		outRule   = iptRule{ipv: ipVer, table: iptables.Filter, chain: "FORWARD", args: []string{"-i", config.BridgeName, "!", "-o", config.BridgeName, "-j", "ACCEPT"}}
		natArgs   []string
		hpNatArgs []string
	)
	hostIP := config.HostIPv4
	nat := !config.GwModeIPv4.routed()
	if ipVer == iptables.IPv6 {
		hostIP = config.HostIPv6
		nat = !config.GwModeIPv6.routed()
	}
	// If hostIP is set, the user wants IPv4/IPv6 SNAT with the given address.
	if hostIP != nil {
		hostAddr := hostIP.String()
		natArgs = []string{"-s", address, "!", "-o", config.BridgeName, "-j", "SNAT", "--to-source", hostAddr}
		hpNatArgs = []string{"-m", "addrtype", "--src-type", "LOCAL", "-o", config.BridgeName, "-j", "SNAT", "--to-source", hostAddr}
		// Else use MASQUERADE which picks the src-ip based on NH from the route table
	} else {
		natArgs = []string{"-s", address, "!", "-o", config.BridgeName, "-j", "MASQUERADE"}
		hpNatArgs = []string{"-m", "addrtype", "--src-type", "LOCAL", "-o", config.BridgeName, "-j", "MASQUERADE"}
	}

	natRule := iptRule{ipv: ipVer, table: iptables.Nat, chain: "POSTROUTING", args: natArgs}
	hpNatRule := iptRule{ipv: ipVer, table: iptables.Nat, chain: "POSTROUTING", args: hpNatArgs}

	// Set NAT.
	if nat && config.EnableIPMasquerade {
		if err := programChainRule(natRule, "NAT", enable); err != nil {
			return err
		}
	}
	if !nat || (config.EnableIPMasquerade && !hairpin) {
		if err := programChainRule(skipDNAT, "SKIP DNAT", enable); err != nil {
			return err
		}
	}

	// In hairpin mode, masquerade traffic from localhost. If hairpin is disabled or if we're tearing down
	// that bridge, make sure the iptables rule isn't lying around.
	if err := programChainRule(hpNatRule, "MASQ LOCAL HOST", enable && hairpin); err != nil {
		return err
	}

	// Set Inter Container Communication.
	if err := setIcc(ipVer, config.BridgeName, config.EnableICC, enable); err != nil {
		return err
	}

	// Allow ICMP in routed mode.
	if !nat {
		if err := setICMP(ipVer, config.BridgeName, enable); err != nil {
			return err
		}
	}

	// Set Accept on all non-intercontainer outgoing packets.
	return programChainRule(outRule, "ACCEPT NON_ICC OUTGOING", enable)
}

func programChainRule(rule iptRule, ruleDescr string, insert bool) error {
	operation := "disable"
	fn := rule.Delete
	if insert {
		operation = "enable"
		fn = rule.Insert
	}
	if err := fn(); err != nil {
		return fmt.Errorf("Unable to %s %s rule: %s", operation, ruleDescr, err.Error())
	}
	return nil
}

func appendOrDelChainRule(rule iptRule, ruleDescr string, append bool) error {
	operation := "disable"
	fn := rule.Delete
	if append {
		operation = "enable"
		fn = rule.Append
	}
	if err := fn(); err != nil {
		return fmt.Errorf("Unable to %s %s rule: %s", operation, ruleDescr, err.Error())
	}
	return nil
}

func setIcc(version iptables.IPVersion, bridgeIface string, iccEnable, insert bool) error {
	args := []string{"-i", bridgeIface, "-o", bridgeIface, "-j"}
	acceptRule := iptRule{ipv: version, table: iptables.Filter, chain: "FORWARD", args: append(args, "ACCEPT")}
	dropRule := iptRule{ipv: version, table: iptables.Filter, chain: "FORWARD", args: append(args, "DROP")}
	if insert {
		if !iccEnable {
			acceptRule.Delete()
			if err := dropRule.Append(); err != nil {
				return fmt.Errorf("Unable to prevent intercontainer communication: %s", err.Error())
			}
		} else {
			dropRule.Delete()
			if err := acceptRule.Insert(); err != nil {
				return fmt.Errorf("Unable to allow intercontainer communication: %s", err.Error())
			}
		}
	} else {
		// Remove any ICC rule.
		if !iccEnable {
			dropRule.Delete()
		} else {
			acceptRule.Delete()
		}
	}
	return nil
}

// Control Inter-Network Communication.
// Install rules only if they aren't present, remove only if they are.
// If this method returns an error, it doesn't roll back any rules it has added.
// No error is returned if rules cannot be removed (errors are just logged).
func setINC(version iptables.IPVersion, iface string, gwm gwMode, enable bool) (retErr error) {
	iptable := iptables.GetIptable(version)
	actionI, actionA := iptables.Insert, iptables.Append
	actionMsg := "add"
	if !enable {
		actionI, actionA = iptables.Delete, iptables.Delete
		actionMsg = "remove"
	}

	if gwm.routed() {
		// Anything is allowed into a routed network at this stage, so RETURN. Port
		// filtering rules in the DOCKER chain will drop anything that's not destined
		// for an open port.
		if err := iptable.ProgramRule(iptables.Filter, IsolationChain1, actionI, []string{
			"-o", iface,
			"-j", "RETURN",
		}); err != nil {
			log.G(context.TODO()).WithError(err).Warnf("Failed to %s inter-network communication rule", actionMsg)
			if enable {
				return fmt.Errorf("%s inter-network communication rule: %w", actionMsg, err)
			}
		}

		// Allow responses from the routed network into whichever network made the request.
		if err := iptable.ProgramRule(iptables.Filter, IsolationChain1, actionI, []string{
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

	if err := iptable.ProgramRule(iptables.Filter, IsolationChain1, actionA, []string{
		"-i", iface,
		"!", "-o", iface,
		"-j", IsolationChain2,
	}); err != nil {
		log.G(context.TODO()).WithError(err).Warnf("Failed to %s inter-network communication rule", actionMsg)
		if enable {
			return fmt.Errorf("%s inter-network communication rule: %w", actionMsg, err)
		}
	}

	if err := iptable.ProgramRule(iptables.Filter, IsolationChain2, actionI, []string{
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
		{Name: DockerChain, Table: iptables.Nat, IPVersion: version},
		{Name: DockerChain, Table: iptables.Filter, IPVersion: version},
		{Name: IsolationChain1, Table: iptables.Filter, IPVersion: version},
		{Name: IsolationChain2, Table: iptables.Filter, IPVersion: version},
		{Name: oldIsolationChain, Table: iptables.Filter, IPVersion: version},
	} {
		if err := chainInfo.Remove(); err != nil {
			log.G(context.TODO()).Warnf("Failed to remove existing iptables entries in table %s chain %s : %v", chainInfo.Table, chainInfo.Name, err)
		}
	}
}

func setupInternalNetworkRules(bridgeIface string, addr *net.IPNet, icc, insert bool) error {
	var version iptables.IPVersion
	var inDropRule, outDropRule iptRule

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

	if addr.IP.To4() != nil {
		version = iptables.IPv4
		inDropRule = iptRule{
			ipv:   version,
			table: iptables.Filter,
			chain: IsolationChain1,
			args:  []string{"-i", bridgeIface, "!", "-d", addr.String(), "-j", "DROP"},
		}
		outDropRule = iptRule{
			ipv:   version,
			table: iptables.Filter,
			chain: IsolationChain1,
			args:  []string{"-o", bridgeIface, "!", "-s", addr.String(), "-j", "DROP"},
		}
	} else {
		version = iptables.IPv6
		inDropRule = iptRule{
			ipv:   version,
			table: iptables.Filter,
			chain: IsolationChain1,
			args:  []string{"-i", bridgeIface, "!", "-o", bridgeIface, "!", "-d", addr.String(), "-j", "DROP"},
		}
		outDropRule = iptRule{
			ipv:   version,
			table: iptables.Filter,
			chain: IsolationChain1,
			args:  []string{"!", "-i", bridgeIface, "-o", bridgeIface, "!", "-s", addr.String(), "-j", "DROP"},
		}
	}

	if err := programChainRule(inDropRule, "DROP INCOMING", insert); err != nil {
		return err
	}
	if err := programChainRule(outDropRule, "DROP OUTGOING", insert); err != nil {
		return err
	}

	// Set Inter Container Communication.
	return setIcc(version, bridgeIface, icc, insert)
}

// clearConntrackEntries flushes conntrack entries matching endpoint IP address
// or matching one of the exposed UDP port.
// In the first case, this could happen if packets were received by the host
// between userland proxy startup and iptables setup.
// In the latter case, this could happen if packets were received whereas there
// were nowhere to route them, as netfilter creates entries in such case.
// This is required because iptables NAT rules are evaluated by netfilter only
// when creating a new conntrack entry. When Docker latter adds NAT rules,
// netfilter ignore them for any packet matching a pre-existing conntrack entry.
// As such, we need to flush all those conntrack entries to make sure NAT rules
// are correctly applied to all packets.
// See: #8795, #44688 & #44742.
func clearConntrackEntries(nlh nlwrap.Handle, ep *bridgeEndpoint) {
	var ipv4List []net.IP
	var ipv6List []net.IP
	var udpPorts []uint16

	if ep.addr != nil {
		ipv4List = append(ipv4List, ep.addr.IP)
	}
	if ep.addrv6 != nil {
		ipv6List = append(ipv6List, ep.addrv6.IP)
	}
	for _, pb := range ep.portMapping {
		if pb.Proto == types.UDP {
			udpPorts = append(udpPorts, pb.HostPort)
		}
	}

	iptables.DeleteConntrackEntries(nlh, ipv4List, ipv6List)
	iptables.DeleteConntrackEntriesByPort(nlh, types.UDP, udpPorts)
}

// mirroredWSL2Workaround adds or removes an IPv4 NAT rule, depending on whether
// docker's host Linux appears to be a guest running under WSL2 in with mirrored
// mode networking.
// https://learn.microsoft.com/en-us/windows/wsl/networking#mirrored-mode-networking
//
// Without mirrored mode networking, or for a packet sent from Linux, packets
// sent to 127.0.0.1 are processed as outgoing - they hit the nat-OUTPUT chain,
// which does not jump to the nat-DOCKER chain because the rule has an exception
// for "-d 127.0.0.0/8". The default action on the nat-OUTPUT chain is ACCEPT (by
// default), so the packet is delivered to 127.0.0.1 on lo, where docker-proxy
// picks it up and acts as a man-in-the-middle; it receives the packet and
// re-sends it to the container (or acks a SYN and sets up a second TCP
// connection to the container). So, the container sees packets arrive with a
// source address belonging to the network's bridge, and it is able to reply to
// that address.
//
// In WSL2's mirrored networking mode, Linux has a loopback0 device as well as lo
// (which owns 127.0.0.1 as normal). Packets sent to 127.0.0.1 from Windows to a
// server listening on Linux's 127.0.0.1 are delivered via loopback0, and
// processed as packets arriving from outside the Linux host (which they are).
//
// So, these packets hit the nat-PREROUTING chain instead of nat-OUTPUT. It would
// normally be impossible for a packet ->127.0.0.1 to arrive from outside the
// host, so the nat-PREROUTING jump to nat-DOCKER has no exception for it. The
// packet is processed by a per-bridge DNAT rule in that chain, so it is
// delivered directly to the container (not via docker-proxy) with source address
// 127.0.0.1, so the container can't respond.
//
// DNAT is normally skipped by RETURN rules in the nat-DOCKER chain for packets
// arriving from any other bridge network. Similarly, this function adds (or
// removes) a rule to RETURN early for packets delivered via loopback0 with
// destination 127.0.0.0/8.
func mirroredWSL2Workaround(config configuration, ipv iptables.IPVersion) error {
	// WSL2 does not (currently) support Windows<->Linux communication via ::1.
	if ipv != iptables.IPv4 {
		return nil
	}
	return programChainRule(mirroredWSL2Rule(), "WSL2 loopback", insertMirroredWSL2Rule(config))
}

// insertMirroredWSL2Rule returns true if the NAT rule for mirrored WSL2 workaround
// is required. It is required if:
//   - the userland proxy is running. If not, there's nothing on the host to catch
//     the packet, so the loopback0 rule as wouldn't be useful. However, without
//     the workaround, with improvements in WSL2 v2.3.11, and without userland proxy
//     running - no workaround is needed, the normal DNAT/masquerading works.
//   - and, the host Linux appears to be running under Windows WSL2 with mirrored
//     mode networking. If a loopback0 device exists, and there's an executable at
//     /usr/bin/wslinfo, infer that this is WSL2 with mirrored networking. ("wslinfo
//     --networking-mode" reports "mirrored", but applying the workaround for WSL2's
//     loopback device when it's not needed is low risk, compared with executing
//     wslinfo with dockerd's elevated permissions.)
func insertMirroredWSL2Rule(config configuration) bool {
	if !config.EnableUserlandProxy || config.UserlandProxyPath == "" {
		return false
	}
	if _, err := nlwrap.LinkByName("loopback0"); err != nil {
		if !errors.As(err, &netlink.LinkNotFoundError{}) {
			log.G(context.TODO()).WithError(err).Warn("Failed to check for WSL interface")
		}
		return false
	}
	stat, err := os.Stat(wslinfoPath)
	if err != nil {
		return false
	}
	return stat.Mode().IsRegular() && (stat.Mode().Perm()&0o111) != 0
}

func mirroredWSL2Rule() iptRule {
	return iptRule{
		ipv:   iptables.IPv4,
		table: iptables.Nat,
		chain: DockerChain,
		args:  []string{"-i", "loopback0", "-d", "127.0.0.0/8", "-j", "RETURN"},
	}
}
