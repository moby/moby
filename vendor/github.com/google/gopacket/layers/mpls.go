// Copyright 2012 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"encoding/binary"
	"errors"
	"github.com/google/gopacket"
)

// MPLS is the MPLS packet header.
type MPLS struct {
	BaseLayer
	Label        uint32
	TrafficClass uint8
	StackBottom  bool
	TTL          uint8
}

// LayerType returns gopacket.LayerTypeMPLS.
func (m *MPLS) LayerType() gopacket.LayerType { return LayerTypeMPLS }

// ProtocolGuessingDecoder attempts to guess the protocol of the bytes it's
// given, then decode the packet accordingly.  Its algorithm for guessing is:
//  If the packet starts with byte 0x45-0x4F: IPv4
//  If the packet starts with byte 0x60-0x6F: IPv6
//  Otherwise:  Error
// See draft-hsmit-isis-aal5mux-00.txt for more detail on this approach.
type ProtocolGuessingDecoder struct{}

func (ProtocolGuessingDecoder) Decode(data []byte, p gopacket.PacketBuilder) error {
	switch data[0] {
	// 0x40 | header_len, where header_len is at least 5.
	case 0x45, 0x46, 0x47, 0x48, 0x49, 0x4a, 0x4b, 0x4c, 0x4d, 0x4e, 0x4f:
		return decodeIPv4(data, p)
		// IPv6 can start with any byte whose first 4 bits are 0x6.
	case 0x60, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6a, 0x6b, 0x6c, 0x6d, 0x6e, 0x6f:
		return decodeIPv6(data, p)
	}
	return errors.New("Unable to guess protocol of packet data")
}

// MPLSPayloadDecoder is the decoder used to data encapsulated by each MPLS
// layer.  MPLS contains no type information, so we have to explicitly decide
// which decoder to use.  This is initially set to ProtocolGuessingDecoder, our
// simple attempt at guessing protocols based on the first few bytes of data
// available to us.  However, if you know that in your environment MPLS always
// encapsulates a specific protocol, you may reset this.
var MPLSPayloadDecoder gopacket.Decoder = ProtocolGuessingDecoder{}

func decodeMPLS(data []byte, p gopacket.PacketBuilder) error {
	decoded := binary.BigEndian.Uint32(data[:4])
	mpls := &MPLS{
		Label:        decoded >> 12,
		TrafficClass: uint8(decoded>>9) & 0x7,
		StackBottom:  decoded&0x100 != 0,
		TTL:          uint8(decoded),
		BaseLayer:    BaseLayer{data[:4], data[4:]},
	}
	p.AddLayer(mpls)
	if mpls.StackBottom {
		return p.NextDecoder(MPLSPayloadDecoder)
	}
	return p.NextDecoder(gopacket.DecodeFunc(decodeMPLS))
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (m *MPLS) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	bytes, err := b.PrependBytes(4)
	if err != nil {
		return err
	}
	encoded := m.Label << 12
	encoded |= uint32(m.TrafficClass) << 9
	encoded |= uint32(m.TTL)
	if m.StackBottom {
		encoded |= 0x100
	}
	binary.BigEndian.PutUint32(bytes, encoded)
	return nil
}
