//go:build linux
// +build linux

package firewalld_test

import (
	"net"
	"strconv"
	"testing"

	"github.com/docker/docker/libnetwork/firewallapi"
	"github.com/docker/docker/libnetwork/firewalld"
	"github.com/docker/docker/libnetwork/iptables"
)

func TestFirewalldInit(t *testing.T) {
	if !firewalld.CheckRunning() {
		t.Skip("firewalld is not running")
	}
	if err := firewalld.FirewalldInit(); err != nil {
		t.Fatal(err)
	}
}

func TestReloaded(t *testing.T) {
	var err error
	var fwdChain firewallapi.FirewallChain

	iptable := iptables.GetTable(iptables.IPv4)
	fwdChain, err = iptable.NewChain("FWD", iptables.Filter, false)
	if err != nil {
		t.Fatal(err)
	}
	bridgeName := "lo"

	err = iptable.ProgramChain(fwdChain, bridgeName, false, true)
	if err != nil {
		t.Fatal(err)
	}
	defer fwdChain.Remove()

	// copy-pasted from iptables_test:TestLink
	ip1 := net.ParseIP("192.168.1.1")
	ip2 := net.ParseIP("192.168.1.2")
	port := 1234
	proto := "tcp"

	err = fwdChain.Link(iptables.Append, ip1, ip2, port, proto, bridgeName)
	if err != nil {
		t.Fatal(err)
	} else {
		// to be re-called again later
		firewalld.OnReloaded(func() { fwdChain.Link(iptables.Append, ip1, ip2, port, proto, bridgeName) })
	}

	rule1 := []string{
		"-i", bridgeName,
		"-o", bridgeName,
		"-p", proto,
		"-s", ip1.String(),
		"-d", ip2.String(),
		"--dport", strconv.Itoa(port),
		"-j", "ACCEPT"}

	if !iptable.Exists(fwdChain.GetTable(), fwdChain.GetName(), rule1...) {
		t.Fatal("rule1 does not exist")
	}

	// flush all rules
	fwdChain.Remove()

	firewalld.Reloaded()

	// make sure the rules have been recreated
	if !iptable.Exists(fwdChain.GetTable(), fwdChain.GetName(), rule1...) {
		t.Fatal("rule1 hasn't been recreated")
	}
}

func TestPassthrough(t *testing.T) {
	rule1 := []string{
		"-i", "lo",
		"-p", "udp",
		"--dport", "123",
		"-j", "ACCEPT"}

	iptable := iptables.GetTable(iptables.IPv4)
	if firewalld.FirewalldRunning {
		_, err := firewalld.Passthrough(firewalld.Iptables, append([]string{"-A"}, rule1...)...)
		if err != nil {
			t.Fatal(err)
		}
		if !iptable.Exists(iptables.Filter, "INPUT", rule1...) {
			t.Fatal("rule1 does not exist")
		}
	}

}
