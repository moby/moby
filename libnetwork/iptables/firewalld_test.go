//go:build linux

package iptables

import (
	"net"
	"strconv"
	"testing"

	"github.com/godbus/dbus/v5"
)

func skipIfNoFirewalld(t *testing.T) {
	t.Helper()
	conn, err := dbus.SystemBus()
	if err != nil {
		t.Skipf("cannot connect to D-bus system bus: %v", err)
	}
	defer conn.Close()

	var zone string
	err = conn.Object(dbusInterface, dbusPath).Call(dbusInterface+".getDefaultZone", 0).Store(&zone)
	if err != nil {
		t.Skipf("firewalld is not running: %v", err)
	}
}

func TestFirewalldInit(t *testing.T) {
	skipIfNoFirewalld(t)
	if err := firewalldInit(); err != nil {
		t.Fatal(err)
	}
}

func TestReloaded(t *testing.T) {
	iptable := GetIptable(IPv4)
	fwdChain, err := iptable.NewChain("FWD", Filter, false)
	if err != nil {
		t.Fatal(err)
	}

	err = iptable.ProgramChain(fwdChain, bridgeName, false, true)
	if err != nil {
		t.Fatal(err)
	}
	defer fwdChain.Remove()

	// copy-pasted from iptables_test:TestLink
	ip1 := net.ParseIP("192.168.1.1")
	ip2 := net.ParseIP("192.168.1.2")
	const port = 1234
	const proto = "tcp"

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
	skipIfNoFirewalld(t)
	rule1 := []string{
		"-i", "lo",
		"-p", "udp",
		"--dport", "123",
		"-j", "ACCEPT",
	}

	_, err := Passthrough(Iptables, append([]string{"-A"}, rule1...)...)
	if err != nil {
		t.Fatal(err)
	}
	if !GetIptable(IPv4).Exists(Filter, "INPUT", rule1...) {
		t.Fatal("rule1 does not exist")
	}
}
