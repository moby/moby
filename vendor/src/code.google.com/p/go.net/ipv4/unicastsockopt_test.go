// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin freebsd linux netbsd openbsd windows

package ipv4_test

import (
	"code.google.com/p/go.net/ipv4"
	"errors"
	"net"
	"os"
	"runtime"
	"testing"
)

type testUnicastConn interface {
	TOS() (int, error)
	SetTOS(int) error
	TTL() (int, error)
	SetTTL(int) error
}

type unicastSockoptTest struct {
	tos int
	ttl int
}

var unicastSockoptTests = []unicastSockoptTest{
	{DiffServCS0 | NotECNTransport, 127},
	{DiffServAF11 | NotECNTransport, 255},
}

func TestTCPUnicastSockopt(t *testing.T) {
	for _, tt := range unicastSockoptTests {
		listener := make(chan net.Listener)
		go tcpListener(t, "127.0.0.1:0", listener)
		ln := <-listener
		if ln == nil {
			return
		}
		defer ln.Close()
		c, err := net.Dial("tcp4", ln.Addr().String())
		if err != nil {
			t.Errorf("net.Dial failed: %v", err)
			return
		}
		defer c.Close()

		cc := ipv4.NewConn(c)
		if err := testUnicastSockopt(t, tt, cc); err != nil {
			break
		}
	}
}

func tcpListener(t *testing.T, addr string, listener chan<- net.Listener) {
	ln, err := net.Listen("tcp4", addr)
	if err != nil {
		t.Errorf("net.Listen failed: %v", err)
		listener <- nil
		return
	}
	listener <- ln
	c, err := ln.Accept()
	if err != nil {
		return
	}
	c.Close()
}

func TestUDPUnicastSockopt(t *testing.T) {
	for _, tt := range unicastSockoptTests {
		c, err := net.ListenPacket("udp4", "127.0.0.1:0")
		if err != nil {
			t.Errorf("net.ListenPacket failed: %v", err)
			return
		}
		defer c.Close()

		p := ipv4.NewPacketConn(c)
		if err := testUnicastSockopt(t, tt, p); err != nil {
			break
		}
	}
}

func TestIPUnicastSockopt(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("must be root")
	}

	for _, tt := range unicastSockoptTests {
		c, err := net.ListenPacket("ip4:icmp", "127.0.0.1")
		if err != nil {
			t.Errorf("net.ListenPacket failed: %v", err)
			return
		}
		defer c.Close()

		r, err := ipv4.NewRawConn(c)
		if err != nil {
			t.Errorf("ipv4.NewRawConn failed: %v", err)
			return
		}
		if err := testUnicastSockopt(t, tt, r); err != nil {
			break
		}
	}
}

func testUnicastSockopt(t *testing.T, tt unicastSockoptTest, c testUnicastConn) error {
	switch runtime.GOOS {
	case "windows":
		// IP_TOS option is supported on Windows 8 and beyond.
		t.Logf("skipping IP_TOS test on %q", runtime.GOOS)
	default:
		if err := c.SetTOS(tt.tos); err != nil {
			t.Errorf("ipv4.Conn.SetTOS failed: %v", err)
			return err
		}
		if v, err := c.TOS(); err != nil {
			t.Errorf("ipv4.Conn.TOS failed: %v", err)
			return err
		} else if v != tt.tos {
			t.Errorf("Got unexpected TOS value %v; expected %v", v, tt.tos)
			return errors.New("Got unexpected TOS value")
		}
	}

	if err := c.SetTTL(tt.ttl); err != nil {
		t.Errorf("ipv4.Conn.SetTTL failed: %v", err)
		return err
	}
	if v, err := c.TTL(); err != nil {
		t.Errorf("ipv4.Conn.TTL failed: %v", err)
		return err
	} else if v != tt.ttl {
		t.Errorf("Got unexpected TTL value %v; expected %v", v, tt.ttl)
		return errors.New("Got unexpected TTL value")
	}

	return nil
}
