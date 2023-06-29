//go:build linux
// +build linux

package iptables

import (
	"net"
	"strconv"
	"testing"
)

func TestFirewalldInit(t *testing.T) {
	if !checkRunning() {
		t.Skip("firewalld is not running")
	}
	if err := FirewalldInit(); err != nil {
		t.Fatal(err)
	}
}

func TestReloaded(t *testing.T) {
	var err error
	var fwdChain *ChainInfo

	iptable := GetIptable(IPv4)
	fwdChain, err = iptable.NewChain("FWD", Filter, false)
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

	err = fwdChain.Link(Append, ip1, ip2, port, proto, bridgeName)
	if err != nil {
		t.Fatal(err)
	} else {
		// to be re-called again later
		OnReloaded(func() { fwdChain.Link(Append, ip1, ip2, port, proto, bridgeName) })
	}

	rule1 := []string{
		"-i", bridgeName,
		"-o", bridgeName,
		"-p", proto,
		"-s", ip1.String(),
		"-d", ip2.String(),
		"--dport", strconv.Itoa(port),
		"-j", "ACCEPT",
	}

	if !iptable.Exists(fwdChain.Table, fwdChain.Name, rule1...) {
		t.Fatal("rule1 does not exist")
	}

	// flush all rules
	fwdChain.Remove()

	reloaded()

	// make sure the rules have been recreated
	if !iptable.Exists(fwdChain.Table, fwdChain.Name, rule1...) {
		t.Fatal("rule1 hasn't been recreated")
	}
}

func TestPassthrough(t *testing.T) {
	rule1 := []string{
		"-i", "lo",
		"-p", "udp",
		"--dport", "123",
		"-j", "ACCEPT",
	}

	iptable := GetIptable(IPv4)
	if firewalldRunning {
		_, err := Passthrough(Iptables, append([]string{"-A"}, rule1...)...)
		if err != nil {
			t.Fatal(err)
		}
		if !iptable.Exists(Filter, "INPUT", rule1...) {
			t.Fatal("rule1 does not exist")
		}
	}
}
