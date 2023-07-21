// Copyright 2012 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"encoding/binary"

	"github.com/google/gopacket"
)

// GRE is a Generic Routing Encapsulation header.
type GRE struct {
	BaseLayer
	ChecksumPresent, RoutingPresent, KeyPresent, SeqPresent, StrictSourceRoute, AckPresent bool
	RecursionControl, Flags, Version                                                       uint8
	Protocol                                                                               EthernetType
	Checksum, Offset                                                                       uint16
	Key, Seq, Ack                                                                          uint32
	*GRERouting
}

// GRERouting is GRE routing information, present if the RoutingPresent flag is
// set.
type GRERouting struct {
	AddressFamily        uint16
	SREOffset, SRELength uint8
	RoutingInformation   []byte
	Next                 *GRERouting
}

// LayerType returns gopacket.LayerTypeGRE.
func (g *GRE) LayerType() gopacket.LayerType { return LayerTypeGRE }

// DecodeFromBytes decodes the given bytes into this layer.
func (g *GRE) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	g.ChecksumPresent = data[0]&0x80 != 0
	g.RoutingPresent = data[0]&0x40 != 0
	g.KeyPresent = data[0]&0x20 != 0
	g.SeqPresent = data[0]&0x10 != 0
	g.StrictSourceRoute = data[0]&0x08 != 0
	g.AckPresent = data[1]&0x80 != 0
	g.RecursionControl = data[0] & 0x7
	g.Flags = data[1] >> 3
	g.Version = data[1] & 0x7
	g.Protocol = EthernetType(binary.BigEndian.Uint16(data[2:4]))
	offset := 4
	if g.ChecksumPresent || g.RoutingPresent {
		g.Checksum = binary.BigEndian.Uint16(data[offset : offset+2])
		g.Offset = binary.BigEndian.Uint16(data[offset+2 : offset+4])
		offset += 4
	}
	if g.KeyPresent {
		g.Key = binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
	}
	if g.SeqPresent {
		g.Seq = binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
	}
	if g.RoutingPresent {
		tail := &g.GRERouting
		for {
			sre := &GRERouting{
				AddressFamily: binary.BigEndian.Uint16(data[offset : offset+2]),
				SREOffset:     data[offset+2],
				SRELength:     data[offset+3],
			}
			sre.RoutingInformation = data[offset+4 : offset+4+int(sre.SRELength)]
			offset += 4 + int(sre.SRELength)
			if sre.AddressFamily == 0 && sre.SRELength == 0 {
				break
			}
			(*tail) = sre
			tail = &sre.Next
		}
	}
	if g.AckPresent {
		g.Ack = binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
	}
	g.BaseLayer = BaseLayer{data[:offset], data[offset:]}
	return nil
}

// SerializeTo writes the serialized form of this layer into the SerializationBuffer,
// implementing gopacket.SerializableLayer. See the docs for gopacket.SerializableLayer for more info.
func (g *GRE) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	size := 4
	if g.ChecksumPresent || g.RoutingPresent {
		size += 4
	}
	if g.KeyPresent {
		size += 4
	}
	if g.SeqPresent {
		size += 4
	}
	if g.RoutingPresent {
		r := g.GRERouting
		for r != nil {
			size += 4 + int(r.SRELength)
			r = r.Next
		}
		size += 4
	}
	if g.AckPresent {
		size += 4
	}
	buf, err := b.PrependBytes(size)
	if err != nil {
		return err
	}
	// Reset any potentially dirty memory in the first 2 bytes, as these use OR to set flags.
	buf[0] = 0
	buf[1] = 0
	if g.ChecksumPresent {
		buf[0] |= 0x80
	}
	if g.RoutingPresent {
		buf[0] |= 0x40
	}
	if g.KeyPresent {
		buf[0] |= 0x20
	}
	if g.SeqPresent {
		buf[0] |= 0x10
	}
	if g.StrictSourceRoute {
		buf[0] |= 0x08
	}
	if g.AckPresent {
		buf[1] |= 0x80
	}
	buf[0] |= g.RecursionControl
	buf[1] |= g.Flags << 3
	buf[1] |= g.Version
	binary.BigEndian.PutUint16(buf[2:4], uint16(g.Protocol))
	offset := 4
	if g.ChecksumPresent || g.RoutingPresent {
		// Don't write the checksum value yet, as we may need to compute it,
		// which requires the entire header be complete.
		// Instead we zeroize the memory in case it is dirty.
		buf[offset] = 0
		buf[offset+1] = 0
		binary.BigEndian.PutUint16(buf[offset+2:offset+4], g.Offset)
		offset += 4
	}
	if g.KeyPresent {
		binary.BigEndian.PutUint32(buf[offset:offset+4], g.Key)
		offset += 4
	}
	if g.SeqPresent {
		binary.BigEndian.PutUint32(buf[offset:offset+4], g.Seq)
		offset += 4
	}
	if g.RoutingPresent {
		sre := g.GRERouting
		for sre != nil {
			binary.BigEndian.PutUint16(buf[offset:offset+2], sre.AddressFamily)
			buf[offset+2] = sre.SREOffset
			buf[offset+3] = sre.SRELength
			copy(buf[offset+4:offset+4+int(sre.SRELength)], sre.RoutingInformation)
			offset += 4 + int(sre.SRELength)
			sre = sre.Next
		}
		// Terminate routing field with a "NULL" SRE.
		binary.BigEndian.PutUint32(buf[offset:offset+4], 0)
	}
	if g.AckPresent {
		binary.BigEndian.PutUint32(buf[offset:offset+4], g.Ack)
		offset += 4
	}
	if g.ChecksumPresent {
		if opts.ComputeChecksums {
			g.Checksum = tcpipChecksum(b.Bytes(), 0)
		}

		binary.BigEndian.PutUint16(buf[4:6], g.Checksum)
	}
	return nil
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (g *GRE) CanDecode() gopacket.LayerClass {
	return LayerTypeGRE
}

// NextLayerType returns the layer type contained by this DecodingLayer.
func (g *GRE) NextLayerType() gopacket.LayerType {
	return g.Protocol.LayerType()
}

func decodeGRE(data []byte, p gopacket.PacketBuilder) error {
	g := &GRE{}
	return decodingLayerDecoder(g, data, p)
}
