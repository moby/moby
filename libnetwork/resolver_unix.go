//go:build !windows
// +build !windows

package libnetwork

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"

	"github.com/docker/docker/libnetwork/firewallapi"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/nftables"
	"github.com/docker/docker/pkg/reexec"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netns"
)

func init() {
	reexec.Register("setup-resolver", reexecSetupResolver)
}

const (
	// outputChain used for docker embed dns
	outputChain = "DOCKER_OUTPUT"
	//postroutingChain used for docker embed dns
	postroutingChain = "DOCKER_POSTROUTING"
)

func reexecSetupResolver() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if len(os.Args) < 4 {
		logrus.Error("invalid number of arguments..")
		os.Exit(1)
	}

	resolverIP, ipPort, _ := net.SplitHostPort(os.Args[2])
	_, tcpPort, _ := net.SplitHostPort(os.Args[3])

	f, err := os.OpenFile(os.Args[1], os.O_RDONLY, 0)
	if err != nil {
		logrus.Errorf("failed get network namespace %q: %v", os.Args[1], err)
		os.Exit(2)
	}
	defer f.Close() //nolint:gosec

	nsFD := f.Fd()
	if err = netns.Set(netns.NsHandle(nsFD)); err != nil {
		logrus.Errorf("setting into container net ns %v failed, %v", os.Args[1], err)
		os.Exit(3)
	}

	// TODO IPv6 support
	var table firewallapi.FirewallTable
	if err := nftables.InitCheck(); err == nil {
		table = nftables.GetTable(nftables.IPv4)
	} else {
		table = iptables.GetTable(iptables.IPv4)
	}

	table.AddJumpRuleForIP(firewallapi.Nat, "OUTPUT", outputChain, resolverIP)
	table.AddJumpRuleForIP(firewallapi.Nat, "POSTROUTING", postroutingChain, resolverIP)

	table.AddDNATwithPort(firewallapi.Nat, outputChain, resolverIP, "udp", dnsPort, os.Args[2])
	table.AddSNATwithPort(firewallapi.Nat, postroutingChain, resolverIP, "udp", ipPort, dnsPort)
	table.AddDNATwithPort(firewallapi.Nat, outputChain, resolverIP, "tcp", dnsPort, os.Args[3])
	table.AddSNATwithPort(firewallapi.Nat, postroutingChain, resolverIP, "tcp", tcpPort, dnsPort)
}

func (r *resolver) setupIPTable() error {
	if r.err != nil {
		return r.err
	}
	laddr := r.conn.LocalAddr().String()
	ltcpaddr := r.tcpListen.Addr().String()

	cmd := &exec.Cmd{
		Path:   reexec.Self(),
		Args:   append([]string{"setup-resolver"}, r.resolverKey, laddr, ltcpaddr),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("reexec failed: %v", err)
	}
	return nil
}
