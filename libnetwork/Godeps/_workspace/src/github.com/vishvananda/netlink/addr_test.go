package netlink

import (
	"net"
	"os"
	"syscall"
	"testing"
)

func TestAddr(t *testing.T) {
	if os.Getenv("TRAVIS_BUILD_DIR") != "" {
		t.Skipf("Fails in travis with: addr_test.go:68: Address flags not set properly, got=0, expected=128")
	}
	// TODO: IFA_F_PERMANENT does not seem to be set by default on older kernels?
	var address = &net.IPNet{net.IPv4(127, 0, 0, 2), net.CIDRMask(24, 32)}
	var addrTests = []struct {
		addr     *Addr
		expected *Addr
	}{
		{
			&Addr{IPNet: address},
			&Addr{IPNet: address, Label: "lo", Scope: syscall.RT_SCOPE_UNIVERSE, Flags: syscall.IFA_F_PERMANENT},
		},
		{
			&Addr{IPNet: address, Label: "local"},
			&Addr{IPNet: address, Label: "local", Scope: syscall.RT_SCOPE_UNIVERSE, Flags: syscall.IFA_F_PERMANENT},
		},
		{
			&Addr{IPNet: address, Flags: syscall.IFA_F_OPTIMISTIC},
			&Addr{IPNet: address, Label: "lo", Flags: syscall.IFA_F_OPTIMISTIC | syscall.IFA_F_PERMANENT, Scope: syscall.RT_SCOPE_UNIVERSE},
		},
		{
			&Addr{IPNet: address, Flags: syscall.IFA_F_OPTIMISTIC | syscall.IFA_F_DADFAILED},
			&Addr{IPNet: address, Label: "lo", Flags: syscall.IFA_F_OPTIMISTIC | syscall.IFA_F_DADFAILED | syscall.IFA_F_PERMANENT, Scope: syscall.RT_SCOPE_UNIVERSE},
		},
		{
			&Addr{IPNet: address, Scope: syscall.RT_SCOPE_NOWHERE},
			&Addr{IPNet: address, Label: "lo", Flags: syscall.IFA_F_PERMANENT, Scope: syscall.RT_SCOPE_NOWHERE},
		},
	}

	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range addrTests {
		if err = AddrAdd(link, tt.addr); err != nil {
			t.Fatal(err)
		}

		addrs, err := AddrList(link, FAMILY_ALL)
		if err != nil {
			t.Fatal(err)
		}

		if len(addrs) != 1 {
			t.Fatal("Address not added properly")
		}

		if !addrs[0].Equal(*tt.expected) {
			t.Fatalf("Address ip no set properly, got=%s, expected=%s", addrs[0], tt.expected)
		}

		if addrs[0].Label != tt.expected.Label {
			t.Fatalf("Address label not set properly, got=%s, expected=%s", addrs[0].Label, tt.expected.Label)
		}

		if addrs[0].Flags != tt.expected.Flags {
			t.Fatalf("Address flags not set properly, got=%d, expected=%d", addrs[0].Flags, tt.expected.Flags)
		}

		if addrs[0].Scope != tt.expected.Scope {
			t.Fatalf("Address scope not set properly, got=%d, expected=%d", addrs[0].Scope, tt.expected.Scope)
		}

		// Pass FAMILY_V4, we should get the same results as FAMILY_ALL
		addrs, err = AddrList(link, FAMILY_V4)
		if err != nil {
			t.Fatal(err)
		}
		if len(addrs) != 1 {
			t.Fatal("Address not added properly")
		}

		// Pass a wrong family number, we should get nil list
		addrs, err = AddrList(link, 0x8)
		if err != nil {
			t.Fatal(err)
		}

		if len(addrs) != 0 {
			t.Fatal("Address not expected")
		}

		if err = AddrDel(link, tt.addr); err != nil {
			t.Fatal(err)
		}

		addrs, err = AddrList(link, FAMILY_ALL)
		if err != nil {
			t.Fatal(err)
		}

		if len(addrs) != 0 {
			t.Fatal("Address not removed properly")
		}
	}
}
