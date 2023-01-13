//go:build !windows
// +build !windows

package libnetwork

import (
	"net"

	"github.com/docker/docker/libnetwork/iptables"
	"github.com/sirupsen/logrus"
)

const (
	// outputChain used for docker embed dns
	outputChain = "DOCKER_OUTPUT"
	//postroutingchain used for docker embed dns
	postroutingchain = "DOCKER_POSTROUTING"
)

func (r *resolver) setupIPTable() error {
	if r.err != nil {
		return r.err
	}
	laddr := r.conn.LocalAddr().String()
	ltcpaddr := r.tcpListen.Addr().String()
	resolverIP, ipPort, _ := net.SplitHostPort(laddr)
	_, tcpPort, _ := net.SplitHostPort(ltcpaddr)
	rules := [][]string{
		{"-t", "nat", "-I", outputChain, "-d", resolverIP, "-p", "udp", "--dport", dnsPort, "-j", "DNAT", "--to-destination", laddr},
		{"-t", "nat", "-I", postroutingchain, "-s", resolverIP, "-p", "udp", "--sport", ipPort, "-j", "SNAT", "--to-source", ":" + dnsPort},
		{"-t", "nat", "-I", outputChain, "-d", resolverIP, "-p", "tcp", "--dport", dnsPort, "-j", "DNAT", "--to-destination", ltcpaddr},
		{"-t", "nat", "-I", postroutingchain, "-s", resolverIP, "-p", "tcp", "--sport", tcpPort, "-j", "SNAT", "--to-source", ":" + dnsPort},
	}

	return r.backend.ExecFunc(func() {
		// TODO IPv6 support
		iptable := iptables.GetIptable(iptables.IPv4)

		// insert outputChain and postroutingchain
		err := iptable.RawCombinedOutputNative("-t", "nat", "-C", "OUTPUT", "-d", resolverIP, "-j", outputChain)
		if err == nil {
			iptable.RawCombinedOutputNative("-t", "nat", "-F", outputChain)
		} else {
			iptable.RawCombinedOutputNative("-t", "nat", "-N", outputChain)
			iptable.RawCombinedOutputNative("-t", "nat", "-I", "OUTPUT", "-d", resolverIP, "-j", outputChain)
		}

		err = iptable.RawCombinedOutputNative("-t", "nat", "-C", "POSTROUTING", "-d", resolverIP, "-j", postroutingchain)
		if err == nil {
			iptable.RawCombinedOutputNative("-t", "nat", "-F", postroutingchain)
		} else {
			iptable.RawCombinedOutputNative("-t", "nat", "-N", postroutingchain)
			iptable.RawCombinedOutputNative("-t", "nat", "-I", "POSTROUTING", "-d", resolverIP, "-j", postroutingchain)
		}

		for _, rule := range rules {
			if iptable.RawCombinedOutputNative(rule...) != nil {
				logrus.Errorf("set up rule failed, %v", rule)
			}
		}
	})
}
