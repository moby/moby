// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ipv4 implements IP-level socket options for the Internet
// Protocol version 4.
//
// The package provides IP-level socket options that allow
// manipulation of IPv4 facilities.  The IPv4 and basic host
// requirements for IPv4 are defined in RFC 791, RFC 1112 and RFC
// 1122.
//
//
// Unicasting
//
// The options for unicasting are available for net.TCPConn,
// net.UDPConn and net.IPConn which are created as network connections
// that use the IPv4 transport.  When a single TCP connection carrying
// a data flow of multiple packets needs to indicate the flow is
// important, ipv4.Conn is used to set the type-of-service field on
// the IPv4 header for each packet.
//
//	ln, err := net.Listen("tcp4", "0.0.0.0:1024")
//	if err != nil {
//		// error handling
//	}
//	defer ln.Close()
//	for {
//		c, err := ln.Accept()
//		if err != nil {
//			// error handling
//		}
//		go func(c net.Conn) {
//			defer c.Close()
//
// The outgoing packets will be labeled DiffServ assured forwarding
// class 1 low drop precedence, as known as AF11 packets.
//
//			if err := ipv4.NewConn(c).SetTOS(DiffServAF11); err != nil {
//				// error handling
//			}
//			if _, err := c.Write(data); err != nil {
//				// error handling
//			}
//		}(c)
//	}
//
//
// Multicasting
//
// The options for multicasting are available for net.UDPConn and
// net.IPconn which are created as network connections that use the
// IPv4 transport.  A few network facilities must be prepared before
// you begin multicasting, at a minimum joining network interfaces and
// multicast groups.
//
//	en0, err := net.InterfaceByName("en0")
//	if err != nil {
//		// error handling
//	}
//	en1, err := net.InterfaceByIndex(911)
//	if err != nil {
//		// error handling
//	}
//	group := net.IPv4(224, 0, 0, 250)
//
// First, an application listens to an appropriate address with an
// appropriate service port.
//
//	c, err := net.ListenPacket("udp4", "0.0.0.0:1024")
//	if err != nil {
//		// error handling
//	}
//	defer c.Close()
//
// Second, the application joins multicast groups, starts listening to
// the groups on the specified network interfaces.  Note that the
// service port for transport layer protocol does not matter with this
// operation as joining groups affects only network and link layer
// protocols, such as IPv4 and Ethernet.
//
//	p := ipv4.NewPacketConn(c)
//	if err := p.JoinGroup(en0, &net.UDPAddr{IP: group}); err != nil {
//		// error handling
//	}
//	if err := p.JoinGroup(en1, &net.UDPAddr{IP: group}); err != nil {
//		// error handling
//	}
//
// The application might set per packet control message transmissions
// between the protocol stack within the kernel.  When the application
// needs a destination address on an incoming packet,
// SetControlMessage of ipv4.PacketConn is used to enable control
// message transmissons.
//
//	if err := p.SetControlMessage(ipv4.FlagDst, true); err != nil {
//		// error handling
//	}
//
// The application could identify whether the received packets are
// of interest by using the control message that contains the
// destination address of the received packet.
//
//	b := make([]byte, 1500)
//	for {
//		n, cm, src, err := p.ReadFrom(b)
//		if err != nil {
//			// error handling
//		}
//		if cm.Dst.IsMulticast() {
//			if cm.Dst.Equal(group)
//				// joined group, do something
//			} else {
//				// unknown group, discard
//				continue
//			}
//		}
//
// The application can also send both unicast and multicast packets.
//
//		p.SetTOS(DiffServCS0)
//		p.SetTTL(16)
//		if _, err := p.WriteTo(data, nil, src); err != nil {
//			// error handling
//		}
//		dst := &net.UDPAddr{IP: group, Port: 1024}
//		for _, ifi := range []*net.Interface{en0, en1} {
//			if err := p.SetMulticastInterface(ifi); err != nil {
//				// error handling
//			}
//			p.SetMulticastTTL(2)
//			if _, err := p.WriteTo(data, nil, dst); err != nil {
//				// error handling
//			}
//		}
//	}
//
//
// More multicasting
//
// An application that uses PacketConn or RawConn may join multiple
// multicast groups.  For example, a UDP listener with port 1024 might
// join two different groups across over two different network
// interfaces by using:
//
//	c, err := net.ListenPacket("udp4", "0.0.0.0:1024")
//	if err != nil {
//		// error handling
//	}
//	defer c.Close()
//	p := ipv4.NewPacketConn(c)
//	if err := p.JoinGroup(en0, &net.UDPAddr{IP: net.IPv4(224, 0, 0, 248)}); err != nil {
//		// error handling
//	}
//	if err := p.JoinGroup(en0, &net.UDPAddr{IP: net.IPv4(224, 0, 0, 249)}); err != nil {
//		// error handling
//	}
//	if err := p.JoinGroup(en1, &net.UDPAddr{IP: net.IPv4(224, 0, 0, 249)}); err != nil {
//		// error handling
//	}
//
// It is possible for multiple UDP listeners that listen on the same
// UDP port to join the same multicast group.  The net package will
// provide a socket that listens to a wildcard address with reusable
// UDP port when an appropriate multicast address prefix is passed to
// the net.ListenPacket or net.ListenUDP.
//
//	c1, err := net.ListenPacket("udp4", "224.0.0.0:1024")
//	if err != nil {
//		// error handling
//	}
//	defer c1.Close()
//	c2, err := net.ListenPacket("udp4", "224.0.0.0:1024")
//	if err != nil {
//		// error handling
//	}
//	defer c2.Close()
//	p1 := ipv4.NewPacketConn(c1)
//	if err := p1.JoinGroup(en0, &net.UDPAddr{IP: net.IPv4(224, 0, 0, 248)}); err != nil {
//		// error handling
//	}
//	p2 := ipv4.NewPacketConn(c2)
//	if err := p2.JoinGroup(en0, &net.UDPAddr{IP: net.IPv4(224, 0, 0, 248)}); err != nil {
//		// error handling
//	}
//
// Also it is possible for the application to leave or rejoin a
// multicast group on the network interface.
//
//	if err := p.LeaveGroup(en0, &net.UDPAddr{IP: net.IPv4(224, 0, 0, 248)}); err != nil {
//		// error handling
//	}
//	if err := p.JoinGroup(en0, &net.UDPAddr{IP: net.IPv4(224, 0, 0, 250)}); err != nil {
//		// error handling
//	}
package ipv4
