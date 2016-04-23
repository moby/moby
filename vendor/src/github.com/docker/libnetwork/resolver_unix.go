// +build !windows

package libnetwork

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/libnetwork/iptables"
	"github.com/vishvananda/netns"
)

func init() {
	reexec.Register("setup-resolver", reexecSetupResolver)
}

func reexecSetupResolver() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if len(os.Args) < 4 {
		log.Error("invalid number of arguments..")
		os.Exit(1)
	}

	_, ipPort, _ := net.SplitHostPort(os.Args[2])
	_, tcpPort, _ := net.SplitHostPort(os.Args[3])
	rules := [][]string{
		{"-t", "nat", "-A", "OUTPUT", "-d", resolverIP, "-p", "udp", "--dport", dnsPort, "-j", "DNAT", "--to-destination", os.Args[2]},
		{"-t", "nat", "-A", "POSTROUTING", "-s", resolverIP, "-p", "udp", "--sport", ipPort, "-j", "SNAT", "--to-source", ":" + dnsPort},
		{"-t", "nat", "-A", "OUTPUT", "-d", resolverIP, "-p", "tcp", "--dport", dnsPort, "-j", "DNAT", "--to-destination", os.Args[3]},
		{"-t", "nat", "-A", "POSTROUTING", "-s", resolverIP, "-p", "tcp", "--sport", tcpPort, "-j", "SNAT", "--to-source", ":" + dnsPort},
	}

	f, err := os.OpenFile(os.Args[1], os.O_RDONLY, 0)
	if err != nil {
		log.Errorf("failed get network namespace %q: %v", os.Args[1], err)
		os.Exit(2)
	}
	defer f.Close()

	nsFD := f.Fd()
	if err = netns.Set(netns.NsHandle(nsFD)); err != nil {
		log.Errorf("setting into container net ns %v failed, %v", os.Args[1], err)
		os.Exit(3)
	}

	for _, rule := range rules {
		if iptables.RawCombinedOutputNative(rule...) != nil {
			log.Errorf("setting up rule failed, %v", rule)
		}
	}
}

func (r *resolver) setupIPTable() error {
	if r.err != nil {
		return r.err
	}
	laddr := r.conn.LocalAddr().String()
	ltcpaddr := r.tcpListen.Addr().String()

	cmd := &exec.Cmd{
		Path:   reexec.Self(),
		Args:   append([]string{"setup-resolver"}, r.sb.Key(), laddr, ltcpaddr),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("reexec failed: %v", err)
	}
	return nil
}
