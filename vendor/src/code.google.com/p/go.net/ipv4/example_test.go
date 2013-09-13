// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ipv4_test

import (
	"code.google.com/p/go.net/ipv4"
	"log"
	"net"
)

func ExampleUnicastTCPListener() {
	ln, err := net.Listen("tcp4", "0.0.0.0:1024")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()
	for {
		c, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func(c net.Conn) {
			defer c.Close()
			err := ipv4.NewConn(c).SetTOS(DiffServAF11)
			if err != nil {
				log.Fatal(err)
			}
			_, err = c.Write([]byte("HELLO-R-U-THERE-ACK"))
			if err != nil {
				log.Fatal(err)
			}
		}(c)
	}
}

func ExampleMulticastUDPListener() {
	en0, err := net.InterfaceByName("en0")
	if err != nil {
		log.Fatal(err)
	}
	en1, err := net.InterfaceByIndex(911)
	if err != nil {
		log.Fatal(err)
	}
	group := net.IPv4(224, 0, 0, 250)

	c, err := net.ListenPacket("udp4", "0.0.0.0:1024")
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	p := ipv4.NewPacketConn(c)
	err = p.JoinGroup(en0, &net.UDPAddr{IP: group})
	if err != nil {
		log.Fatal(err)
	}
	err = p.JoinGroup(en1, &net.UDPAddr{IP: group})
	if err != nil {
		log.Fatal(err)
	}

	err = p.SetControlMessage(ipv4.FlagDst, true)
	if err != nil {
		log.Fatal(err)
	}

	b := make([]byte, 1500)
	for {
		n, cm, src, err := p.ReadFrom(b)
		if err != nil {
			log.Fatal(err)
		}
		if cm.Dst.IsMulticast() {
			if cm.Dst.Equal(group) {
				// joined group, do something
			} else {
				// unknown group, discard
				continue
			}
		}
		p.SetTOS(DiffServCS7)
		p.SetTTL(16)
		_, err = p.WriteTo(b[:n], nil, src)
		if err != nil {
			log.Fatal(err)
		}
		dst := &net.UDPAddr{IP: group, Port: 1024}
		for _, ifi := range []*net.Interface{en0, en1} {
			err := p.SetMulticastInterface(ifi)
			if err != nil {
				log.Fatal(err)
			}
			p.SetMulticastTTL(2)
			_, err = p.WriteTo(b[:n], nil, dst)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	err = p.LeaveGroup(en1, &net.UDPAddr{IP: group})
	if err != nil {
		log.Fatal(err)
	}
	newgroup := net.IPv4(224, 0, 0, 249)
	err = p.JoinGroup(en1, &net.UDPAddr{IP: newgroup})
	if err != nil {
		log.Fatal(err)
	}
}

type OSPFHeader struct {
	Version  byte
	Type     byte
	Len      uint16
	RouterID uint32
	AreaID   uint32
	Checksum uint16
}

const (
	OSPFHeaderLen      = 14
	OSPFHelloHeaderLen = 20
	OSPF_VERSION       = 2
	OSPF_TYPE_HELLO    = iota + 1
	OSPF_TYPE_DB_DESCRIPTION
	OSPF_TYPE_LS_REQUEST
	OSPF_TYPE_LS_UPDATE
	OSPF_TYPE_LS_ACK
)

var (
	AllSPFRouters = net.IPv4(224, 0, 0, 5)
	AllDRouters   = net.IPv4(224, 0, 0, 6)
)

func ExampleIPOSPFListener() {
	var ifs []*net.Interface
	en0, err := net.InterfaceByName("en0")
	if err != nil {
		log.Fatal(err)
	}
	ifs = append(ifs, en0)
	en1, err := net.InterfaceByIndex(911)
	if err != nil {
		log.Fatal(err)
	}
	ifs = append(ifs, en1)

	c, err := net.ListenPacket("ip4:89", "0.0.0.0") // OSFP for IPv4
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	r, err := ipv4.NewRawConn(c)
	if err != nil {
		log.Fatal(err)
	}
	for _, ifi := range ifs {
		err := r.JoinGroup(ifi, &net.IPAddr{IP: AllSPFRouters})
		if err != nil {
			log.Fatal(err)
		}
		err = r.JoinGroup(ifi, &net.IPAddr{IP: AllDRouters})
		if err != nil {
			log.Fatal(err)
		}
	}

	err = r.SetControlMessage(ipv4.FlagDst|ipv4.FlagInterface, true)
	if err != nil {
		log.Fatal(err)
	}
	r.SetTOS(DiffServCS6)

	parseOSPFHeader := func(b []byte) *OSPFHeader {
		if len(b) < OSPFHeaderLen {
			return nil
		}
		return &OSPFHeader{
			Version:  b[0],
			Type:     b[1],
			Len:      uint16(b[2])<<8 | uint16(b[3]),
			RouterID: uint32(b[4])<<24 | uint32(b[5])<<16 | uint32(b[6])<<8 | uint32(b[7]),
			AreaID:   uint32(b[8])<<24 | uint32(b[9])<<16 | uint32(b[10])<<8 | uint32(b[11]),
			Checksum: uint16(b[12])<<8 | uint16(b[13]),
		}
	}

	b := make([]byte, 1500)
	for {
		iph, p, _, err := r.ReadFrom(b)
		if err != nil {
			log.Fatal(err)
		}
		if iph.Version != ipv4.Version {
			continue
		}
		if iph.Dst.IsMulticast() {
			if !iph.Dst.Equal(AllSPFRouters) && !iph.Dst.Equal(AllDRouters) {
				continue
			}
		}
		ospfh := parseOSPFHeader(p)
		if ospfh == nil {
			continue
		}
		if ospfh.Version != OSPF_VERSION {
			continue
		}
		switch ospfh.Type {
		case OSPF_TYPE_HELLO:
		case OSPF_TYPE_DB_DESCRIPTION:
		case OSPF_TYPE_LS_REQUEST:
		case OSPF_TYPE_LS_UPDATE:
		case OSPF_TYPE_LS_ACK:
		}
	}
}

func ExampleWriteIPOSPFHello() {
	var ifs []*net.Interface
	en0, err := net.InterfaceByName("en0")
	if err != nil {
		log.Fatal(err)
	}
	ifs = append(ifs, en0)
	en1, err := net.InterfaceByIndex(911)
	if err != nil {
		log.Fatal(err)
	}
	ifs = append(ifs, en1)

	c, err := net.ListenPacket("ip4:89", "0.0.0.0") // OSPF for IPv4
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	r, err := ipv4.NewRawConn(c)
	if err != nil {
		log.Fatal(err)
	}
	for _, ifi := range ifs {
		err := r.JoinGroup(ifi, &net.IPAddr{IP: AllSPFRouters})
		if err != nil {
			log.Fatal(err)
		}
		err = r.JoinGroup(ifi, &net.IPAddr{IP: AllDRouters})
		if err != nil {
			log.Fatal(err)
		}
	}

	hello := make([]byte, OSPFHelloHeaderLen)
	ospf := make([]byte, OSPFHeaderLen)
	ospf[0] = OSPF_VERSION
	ospf[1] = OSPF_TYPE_HELLO
	ospf = append(ospf, hello...)
	iph := &ipv4.Header{}
	iph.Version = ipv4.Version
	iph.Len = ipv4.HeaderLen
	iph.TOS = DiffServCS6
	iph.TotalLen = ipv4.HeaderLen + len(ospf)
	iph.TTL = 1
	iph.Protocol = 89
	iph.Dst = AllSPFRouters

	for _, ifi := range ifs {
		err := r.SetMulticastInterface(ifi)
		if err != nil {
			return
		}
		err = r.WriteTo(iph, ospf, nil)
		if err != nil {
			return
		}
	}
}
