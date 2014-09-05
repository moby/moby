// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin freebsd linux netbsd openbsd windows

package ipv4_test

import (
	"code.google.com/p/go.net/ipv4"
	"net"
	"os"
	"runtime"
	"testing"
)

type testMulticastConn interface {
	testUnicastConn
	MulticastTTL() (int, error)
	SetMulticastTTL(ttl int) error
	MulticastLoopback() (bool, error)
	SetMulticastLoopback(bool) error
	JoinGroup(*net.Interface, net.Addr) error
	LeaveGroup(*net.Interface, net.Addr) error
}

type multicastSockoptTest struct {
	tos    int
	ttl    int
	mcttl  int
	mcloop bool
	gaddr  net.IP
}

var multicastSockoptTests = []multicastSockoptTest{
	{DiffServCS0 | NotECNTransport, 127, 128, false, net.IPv4(224, 0, 0, 249)}, // see RFC 4727
	{DiffServAF11 | NotECNTransport, 255, 254, true, net.IPv4(224, 0, 0, 250)}, // see RFC 4727
}

func TestUDPMulticastSockopt(t *testing.T) {
	if testing.Short() || !*testExternal {
		t.Skip("to avoid external network")
	}

	for _, tt := range multicastSockoptTests {
		c, err := net.ListenPacket("udp4", "0.0.0.0:0")
		if err != nil {
			t.Fatalf("net.ListenPacket failed: %v", err)
		}
		defer c.Close()

		p := ipv4.NewPacketConn(c)
		testMulticastSockopt(t, tt, p, &net.UDPAddr{IP: tt.gaddr})
	}
}

func TestIPMulticastSockopt(t *testing.T) {
	if testing.Short() || !*testExternal {
		t.Skip("to avoid external network")
	}
	if os.Getuid() != 0 {
		t.Skip("must be root")
	}

	for _, tt := range multicastSockoptTests {
		c, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
		if err != nil {
			t.Fatalf("net.ListenPacket failed: %v", err)
		}
		defer c.Close()

		r, _ := ipv4.NewRawConn(c)
		testMulticastSockopt(t, tt, r, &net.IPAddr{IP: tt.gaddr})
	}
}

func testMulticastSockopt(t *testing.T, tt multicastSockoptTest, c testMulticastConn, gaddr net.Addr) {
	switch runtime.GOOS {
	case "windows":
		// IP_TOS option is supported on Windows 8 and beyond.
		t.Logf("skipping IP_TOS test on %q", runtime.GOOS)
	default:
		if err := c.SetTOS(tt.tos); err != nil {
			t.Fatalf("ipv4.PacketConn.SetTOS failed: %v", err)
		}
		if v, err := c.TOS(); err != nil {
			t.Fatalf("ipv4.PacketConn.TOS failed: %v", err)
		} else if v != tt.tos {
			t.Fatalf("Got unexpected TOS value %v; expected %v", v, tt.tos)
		}
	}

	if err := c.SetTTL(tt.ttl); err != nil {
		t.Fatalf("ipv4.PacketConn.SetTTL failed: %v", err)
	}
	if v, err := c.TTL(); err != nil {
		t.Fatalf("ipv4.PacketConn.TTL failed: %v", err)
	} else if v != tt.ttl {
		t.Fatalf("Got unexpected TTL value %v; expected %v", v, tt.ttl)
	}

	if err := c.SetMulticastTTL(tt.mcttl); err != nil {
		t.Fatalf("ipv4.PacketConn.SetMulticastTTL failed: %v", err)
	}
	if v, err := c.MulticastTTL(); err != nil {
		t.Fatalf("ipv4.PacketConn.MulticastTTL failed: %v", err)
	} else if v != tt.mcttl {
		t.Fatalf("Got unexpected MulticastTTL value %v; expected %v", v, tt.mcttl)
	}

	if err := c.SetMulticastLoopback(tt.mcloop); err != nil {
		t.Fatalf("ipv4.PacketConn.SetMulticastLoopback failed: %v", err)
	}
	if v, err := c.MulticastLoopback(); err != nil {
		t.Fatalf("ipv4.PacketConn.MulticastLoopback failed: %v", err)
	} else if v != tt.mcloop {
		t.Fatalf("Got unexpected MulticastLoopback value %v; expected %v", v, tt.mcloop)
	}

	if err := c.JoinGroup(nil, gaddr); err != nil {
		t.Fatalf("ipv4.PacketConn.JoinGroup(%v) failed: %v", gaddr, err)
	}
	if err := c.LeaveGroup(nil, gaddr); err != nil {
		t.Fatalf("ipv4.PacketConn.LeaveGroup(%v) failed: %v", gaddr, err)
	}
}
