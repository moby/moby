//go:build !windows

package libnetwork

import (
	"context"
	"fmt"
	"net"

	"github.com/moby/moby/v2/daemon/libnetwork/internal/nftables"
	"github.com/moby/moby/v2/daemon/libnetwork/iptables"
)

const (
	// output chain used for docker embedded DNS resolver
	outputChain = "DOCKER_OUTPUT"
	// postrouting chain used for docker embedded DNS resolver
	postroutingChain = "DOCKER_POSTROUTING"
)

func (r *Resolver) setupNAT(ctx context.Context) error {
	if r.err != nil {
		return r.err
	}
	laddr := r.conn.LocalAddr().String()
	ltcpaddr := r.tcpListen.Addr().String()
	resolverIP, ipPort, _ := net.SplitHostPort(laddr)
	_, tcpPort, _ := net.SplitHostPort(ltcpaddr)

	if nftables.Enabled() {
		return r.setupNftablesNAT(ctx, laddr, ltcpaddr, resolverIP, ipPort, tcpPort)
	}
	return r.setupIptablesNAT(laddr, ltcpaddr, resolverIP, ipPort, tcpPort)
}

func (r *Resolver) setupIptablesNAT(laddr, ltcpaddr, resolverIP, ipPort, tcpPort string) error {
	rules := [][]string{
		{"-t", "nat", "-I", outputChain, "-d", resolverIP, "-p", "udp", "--dport", dnsPort, "-j", "DNAT", "--to-destination", laddr},
		{"-t", "nat", "-I", postroutingChain, "-s", resolverIP, "-p", "udp", "--sport", ipPort, "-j", "SNAT", "--to-source", ":" + dnsPort},
		{"-t", "nat", "-I", outputChain, "-d", resolverIP, "-p", "tcp", "--dport", dnsPort, "-j", "DNAT", "--to-destination", ltcpaddr},
		{"-t", "nat", "-I", postroutingChain, "-s", resolverIP, "-p", "tcp", "--sport", tcpPort, "-j", "SNAT", "--to-source", ":" + dnsPort},
	}

	var setupErr error
	err := r.backend.ExecFunc(func() {
		// TODO IPv6 support
		iptable := iptables.GetIptable(iptables.IPv4)

		// insert outputChain and postroutingchain
		if iptable.ExistsNative("nat", "OUTPUT", "-d", resolverIP, "-j", outputChain) {
			if err := iptable.RawCombinedOutputNative("-t", "nat", "-F", outputChain); err != nil {
				setupErr = err
				return
			}
		} else {
			if err := iptable.RawCombinedOutputNative("-t", "nat", "-N", outputChain); err != nil {
				setupErr = err
				return
			}
			if err := iptable.RawCombinedOutputNative("-t", "nat", "-I", "OUTPUT", "-d", resolverIP, "-j", outputChain); err != nil {
				setupErr = err
				return
			}
		}

		if iptable.ExistsNative("nat", "POSTROUTING", "-d", resolverIP, "-j", postroutingChain) {
			if err := iptable.RawCombinedOutputNative("-t", "nat", "-F", postroutingChain); err != nil {
				setupErr = err
				return
			}
		} else {
			if err := iptable.RawCombinedOutputNative("-t", "nat", "-N", postroutingChain); err != nil {
				setupErr = err
				return
			}
			if err := iptable.RawCombinedOutputNative("-t", "nat", "-I", "POSTROUTING", "-d", resolverIP, "-j", postroutingChain); err != nil {
				setupErr = err
				return
			}
		}

		for _, rule := range rules {
			if iptable.RawCombinedOutputNative(rule...) != nil {
				setupErr = fmt.Errorf("set up rule failed, %v", rule)
				return
			}
		}
	})
	if err != nil {
		return err
	}
	return setupErr
}

func (r *Resolver) setupNftablesNAT(ctx context.Context, laddr, ltcpaddr, resolverIP, ipPort, tcpPort string) error {
	table, err := nftables.NewTable(nftables.IPv4, "docker-dns")
	if err != nil {
		return err
	}

	dnatChain, err := table.BaseChain(ctx, "dns-dnat", nftables.BaseChainTypeNAT, nftables.BaseChainHookOutput, nftables.BaseChainPriorityDstNAT)
	if err != nil {
		return err
	}
	if err := dnatChain.AppendRule(ctx, 0, "ip daddr %s udp dport %s counter dnat to %s", resolverIP, dnsPort, laddr); err != nil {
		return err
	}
	if err := dnatChain.AppendRule(ctx, 0, "ip daddr %s tcp dport %s counter dnat to %s", resolverIP, dnsPort, ltcpaddr); err != nil {
		return err
	}

	snatChain, err := table.BaseChain(ctx, "dns-snat", nftables.BaseChainTypeNAT, nftables.BaseChainHookPostrouting, nftables.BaseChainPrioritySrcNAT)
	if err != nil {
		return err
	}
	if err := snatChain.AppendRule(ctx, 0, "ip saddr %s udp sport %s counter snat to :%s", resolverIP, ipPort, dnsPort); err != nil {
		return err
	}
	if err := snatChain.AppendRule(ctx, 0, "ip saddr %s tcp sport %s counter snat to :%s", resolverIP, tcpPort, dnsPort); err != nil {
		return err
	}

	var setupErr error
	if err := r.backend.ExecFunc(func() {
		setupErr = table.Apply(ctx)
	}); err != nil {
		return err
	}
	return setupErr
}
