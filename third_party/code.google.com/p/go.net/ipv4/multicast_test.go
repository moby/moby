// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin freebsd linux netbsd openbsd

package ipv4_test

import (
	"code.google.com/p/go.net/ipv4"
	"net"
	"os"
	"testing"
)

func TestReadWriteMulticastIPPayloadUDP(t *testing.T) {
	if testing.Short() || !*testExternal {
		t.Skip("to avoid external network")
	}

	c, err := net.ListenPacket("udp4", "224.0.0.0:1024") // see RFC 4727
	if err != nil {
		t.Fatalf("net.ListenPacket failed: %v", err)
	}
	defer c.Close()

	ifi := loopbackInterface()
	if ifi == nil {
		t.Skip("an appropriate interface not found")
	}
	dst, err := net.ResolveUDPAddr("udp4", "224.0.0.254:1024") // see RFC 4727
	if err != nil {
		t.Fatalf("net.ResolveUDPAddr failed: %v", err)
	}

	p := ipv4.NewPacketConn(c)
	if err := p.JoinGroup(ifi, dst); err != nil {
		t.Fatalf("ipv4.PacketConn.JoinGroup on %v failed: %v", ifi, err)
	}
	if err := p.SetMulticastInterface(ifi); err != nil {
		t.Fatalf("ipv4.PacketConn.SetMulticastInterface failed: %v", err)
	}
	if err := p.SetMulticastLoopback(true); err != nil {
		t.Fatalf("ipv4.PacketConn.SetMulticastLoopback failed: %v", err)
	}
	cf := ipv4.FlagTTL | ipv4.FlagDst | ipv4.FlagInterface
	for i, toggle := range []bool{true, false, true} {
		if err := p.SetControlMessage(cf, toggle); err != nil {
			t.Fatalf("ipv4.PacketConn.SetControlMessage failed: %v", err)
		}
		writeThenReadPayload(t, i, p, []byte("HELLO-R-U-THERE"), dst)
	}
}

func TestReadWriteMulticastIPPayloadICMP(t *testing.T) {
	if testing.Short() || !*testExternal {
		t.Skip("to avoid external network")
	}
	if os.Getuid() != 0 {
		t.Skip("must be root")
	}

	c, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		t.Fatalf("net.ListenPacket failed: %v", err)
	}
	defer c.Close()

	ifi := loopbackInterface()
	if ifi == nil {
		t.Skip("an appropriate interface not found")
	}
	dst, err := net.ResolveIPAddr("ip4", "224.0.0.254") // see RFC 4727
	if err != nil {
		t.Fatalf("net.ResolveIPAddr failed: %v", err)
	}

	p := ipv4.NewPacketConn(c)
	if err := p.JoinGroup(ifi, dst); err != nil {
		t.Fatalf("ipv4.PacketConn.JoinGroup on %v failed: %v", ifi, err)
	}
	if err := p.SetMulticastInterface(ifi); err != nil {
		t.Fatalf("ipv4.PacketConn.SetMulticastInterface failed: %v", err)
	}
	cf := ipv4.FlagTTL | ipv4.FlagDst | ipv4.FlagInterface
	for i, toggle := range []bool{true, false, true} {
		wb, err := (&icmpMessage{
			Type: ipv4.ICMPTypeEcho, Code: 0,
			Body: &icmpEcho{
				ID: os.Getpid() & 0xffff, Seq: i + 1,
				Data: []byte("HELLO-R-U-THERE"),
			},
		}).Marshal()
		if err != nil {
			t.Fatalf("icmpMessage.Marshal failed: %v", err)
		}
		if err := p.SetControlMessage(cf, toggle); err != nil {
			t.Fatalf("ipv4.PacketConn.SetControlMessage failed: %v", err)
		}
		rb := writeThenReadPayload(t, i, p, wb, dst)
		m, err := parseICMPMessage(rb)
		if err != nil {
			t.Fatalf("parseICMPMessage failed: %v", err)
		}
		if m.Type != ipv4.ICMPTypeEchoReply || m.Code != 0 {
			t.Fatalf("got type=%v, code=%v; expected type=%v, code=%v", m.Type, m.Code, ipv4.ICMPTypeEchoReply, 0)
		}
	}
}

func TestReadWriteMulticastIPDatagram(t *testing.T) {
	if testing.Short() || !*testExternal {
		t.Skip("to avoid external network")
	}
	if os.Getuid() != 0 {
		t.Skip("must be root")
	}

	c, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		t.Fatalf("net.ListenPacket failed: %v", err)
	}
	defer c.Close()

	ifi := loopbackInterface()
	if ifi == nil {
		t.Skip("an appropriate interface not found")
	}
	dst, err := net.ResolveIPAddr("ip4", "224.0.0.254") // see RFC 4727
	if err != nil {
		t.Fatalf("ResolveIPAddr failed: %v", err)
	}

	r, err := ipv4.NewRawConn(c)
	if err != nil {
		t.Fatalf("ipv4.NewRawConn failed: %v", err)
	}
	if err := r.JoinGroup(ifi, dst); err != nil {
		t.Fatalf("ipv4.RawConn.JoinGroup on %v failed: %v", ifi, err)
	}
	if err := r.SetMulticastInterface(ifi); err != nil {
		t.Fatalf("ipv4.PacketConn.SetMulticastInterface failed: %v", err)
	}
	cf := ipv4.FlagTTL | ipv4.FlagDst | ipv4.FlagInterface
	for i, toggle := range []bool{true, false, true} {
		wb, err := (&icmpMessage{
			Type: ipv4.ICMPTypeEcho, Code: 0,
			Body: &icmpEcho{
				ID: os.Getpid() & 0xffff, Seq: i + 1,
				Data: []byte("HELLO-R-U-THERE"),
			},
		}).Marshal()
		if err != nil {
			t.Fatalf("icmpMessage.Marshal failed: %v", err)
		}
		if err := r.SetControlMessage(cf, toggle); err != nil {
			t.Fatalf("ipv4.RawConn.SetControlMessage failed: %v", err)
		}
		rb := writeThenReadDatagram(t, i, r, wb, nil, dst)
		m, err := parseICMPMessage(rb)
		if err != nil {
			t.Fatalf("parseICMPMessage failed: %v", err)
		}
		if m.Type != ipv4.ICMPTypeEchoReply || m.Code != 0 {
			t.Fatalf("got type=%v, code=%v; expected type=%v, code=%v", m.Type, m.Code, ipv4.ICMPTypeEchoReply, 0)
		}
	}
}
