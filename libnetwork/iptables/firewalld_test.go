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
	fwd, err := firewalldInit()
	if err != nil {
		t.Fatal(err)
	}
	_ = fwd.conn.Close()
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

	// create a dummy firewalldConnection and mark it as "running", because
	// OnReloaded (registerReloadCallback),
	fwd := &firewalldConnection{}
	fwd.running.Store(true)

	err = fwdChain.Link(Append, ip1, ip2, port, proto, bridgeName)
	if err != nil {
		t.Fatal(err)
	} else {
		// to be re-called again later
		fwd.registerReloadCallback(func() { fwdChain.Link(Append, ip1, ip2, port, proto, bridgeName) })
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

	fwd.onReload()

	// make sure the rules have been recreated
	if !iptable.Exists(fwdChain.Table, fwdChain.Name, rule1...) {
		t.Fatal("rule1 hasn't been recreated")
	}
}

func TestPassthrough(t *testing.T) {
	skipIfNoFirewalld(t)

	fwd, err := newConnection()
	if err != nil {
		t.Fatal(err)
	}
	defer fwd.conn.Close()

	rule1 := []string{
		"-i", "lo",
		"-p", "udp",
		"--dport", "123",
		"-j", "ACCEPT",
	}

	_, err = fwd.passthrough(IPv4, append([]string{"-A"}, rule1...)...)
	if err != nil {
		t.Error(err)
	}
	if !GetIptable(IPv4).Exists(Filter, "INPUT", rule1...) {
		t.Error("rule1 does not exist")
	}
}

// TestFirewalldUninitialized checks that calling methods, such as isRunning()
// on an empty, uninitialized firewalldConnection doesn't panic, and returns
// the expected status.
func TestFirewalldUninitialized(t *testing.T) {
	var fwd *firewalldConnection
	if fwd.isRunning() {
		t.Error("did not expect an uninitialized firewalldConnection to be running")
	}
	err := fwd.addInterface("anything")
	if err != nil {
		t.Errorf("unexpected error when calling addInterface on an uninitialized firewalldConnection: %v", err)
	}
	err = fwd.delInterface("anything")
	if err != nil {
		t.Errorf("unexpected error when calling delInterface on an uninitialized firewalldConnection: %v", err)
	}
	fwd.registerReloadCallback(func() {})
	_, err = fwd.passthrough(IPv4)
	if err != nil {
		t.Errorf("unexpected error when calling passthrough on an uninitialized firewalldConnection: %v", err)
	}
}
