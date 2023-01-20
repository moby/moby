//go:build !windows
// +build !windows

package libnetwork

import (
	"fmt"
	"net"

	"github.com/docker/docker/libnetwork/iptables"
)

const (
	// output chain used for docker embedded DNS resolver
	outputChain = "DOCKER_OUTPUT"
	// postrouting chain used for docker embedded DNS resolver
	postroutingChain = "DOCKER_POSTROUTING"
)

func (r *Resolver) setupIPTable() error {
	if r.err != nil {
		return r.err
	}
	laddr := r.conn.LocalAddr().String()
	ltcpaddr := r.tcpListen.Addr().String()
	resolverIP, ipPort, _ := net.SplitHostPort(laddr)
	_, tcpPort, _ := net.SplitHostPort(ltcpaddr)
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
		err := iptable.RawCombinedOutputNative("-t", "nat", "-C", "OUTPUT", "-d", resolverIP, "-j", outputChain)
		if err == nil {
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

		err = iptable.RawCombinedOutputNative("-t", "nat", "-C", "POSTROUTING", "-d", resolverIP, "-j", postroutingChain)
		if err == nil {
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
