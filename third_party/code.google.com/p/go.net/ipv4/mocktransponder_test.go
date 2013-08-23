// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin freebsd linux netbsd openbsd

package ipv4_test

import (
	"code.google.com/p/go.net/ipv4"
	"net"
	"testing"
	"time"
)

// writeThenReadPayload transmits IPv4 datagram payloads to the
// loopback address or interface and captures the loopback'd datagram
// payloads.
func writeThenReadPayload(t *testing.T, i int, c *ipv4.PacketConn, wb []byte, dst net.Addr) []byte {
	rb := make([]byte, 1500)
	c.SetTOS(i + 1)
	var ip net.IP
	switch v := dst.(type) {
	case *net.UDPAddr:
		ip = v.IP
	case *net.IPAddr:
		ip = v.IP
	}
	if ip.IsMulticast() {
		c.SetMulticastTTL(i + 1)
	} else {
		c.SetTTL(i + 1)
	}
	c.SetDeadline(time.Now().Add(100 * time.Millisecond))
	if _, err := c.WriteTo(wb, nil, dst); err != nil {
		t.Fatalf("ipv4.PacketConn.WriteTo failed: %v", err)
	}
	n, cm, _, err := c.ReadFrom(rb)
	if err != nil {
		t.Fatalf("ipv4.PacketConn.ReadFrom failed: %v", err)
	}
	t.Logf("rcvd cmsg: %v", cm)
	return rb[:n]
}

// writeThenReadDatagram transmits ICMP for IPv4 datagrams to the
// loopback address or interface and captures the response datagrams
// from the protocol stack within the kernel.
func writeThenReadDatagram(t *testing.T, i int, c *ipv4.RawConn, wb []byte, src, dst net.Addr) []byte {
	rb := make([]byte, ipv4.HeaderLen+len(wb))
	wh := &ipv4.Header{
		Version:  ipv4.Version,
		Len:      ipv4.HeaderLen,
		TOS:      i + 1,
		TotalLen: ipv4.HeaderLen + len(wb),
		TTL:      i + 1,
		Protocol: 1,
	}
	if src != nil {
		wh.Src = src.(*net.IPAddr).IP
	}
	if dst != nil {
		wh.Dst = dst.(*net.IPAddr).IP
	}
	c.SetDeadline(time.Now().Add(100 * time.Millisecond))
	if err := c.WriteTo(wh, wb, nil); err != nil {
		t.Fatalf("ipv4.RawConn.WriteTo failed: %v", err)
	}
	rh, b, cm, err := c.ReadFrom(rb)
	if err != nil {
		t.Fatalf("ipv4.RawConn.ReadFrom failed: %v", err)
	}
	t.Logf("rcvd cmsg: %v", cm.String())
	t.Logf("rcvd hdr: %v", rh.String())
	return b
}

// LoopbackInterface returns a logical network interface for loopback
// tests.
func loopbackInterface() *net.Interface {
	ift, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, ifi := range ift {
		if ifi.Flags&net.FlagLoopback != 0 {
			return &ifi
		}
	}
	return nil
}

// isMulticastAvailable returns true if ifi is a multicast access
// enabled network interface.  It also returns a unicast IPv4 address
// that can be used for listening on ifi.
func isMulticastAvailable(ifi *net.Interface) (net.IP, bool) {
	if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagMulticast == 0 {
		return nil, false
	}
	ifat, err := ifi.Addrs()
	if err != nil {
		return nil, false
	}
	if len(ifat) == 0 {
		return nil, false
	}
	var ip net.IP
	for _, ifa := range ifat {
		switch v := ifa.(type) {
		case *net.IPAddr:
			ip = v.IP
		case *net.IPNet:
			ip = v.IP
		default:
			continue
		}
		if ip.To4() == nil {
			ip = nil
			continue
		}
		break
	}
	return ip, true
}
