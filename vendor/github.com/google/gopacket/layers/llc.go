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

// LLC is the layer used for 802.2 Logical Link Control headers.
// See http://standards.ieee.org/getieee802/download/802.2-1998.pdf
type LLC struct {
	BaseLayer
	DSAP    uint8
	IG      bool // true means group, false means individual
	SSAP    uint8
	CR      bool // true means response, false means command
	Control uint16
}

// LayerType returns gopacket.LayerTypeLLC.
func (l *LLC) LayerType() gopacket.LayerType { return LayerTypeLLC }

// DecodeFromBytes decodes the given bytes into this layer.
func (l *LLC) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 3 {
		return errors.New("LLC header too small")
	}
	l.DSAP = data[0] & 0xFE
	l.IG = data[0]&0x1 != 0
	l.SSAP = data[1] & 0xFE
	l.CR = data[1]&0x1 != 0
	l.Control = uint16(data[2])

	if l.Control&0x1 == 0 || l.Control&0x3 == 0x1 {
		if len(data) < 4 {
			return errors.New("LLC header too small")
		}
		l.Control = l.Control<<8 | uint16(data[3])
		l.Contents = data[:4]
		l.Payload = data[4:]
	} else {
		l.Contents = data[:3]
		l.Payload = data[3:]
	}
	return nil
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (l *LLC) CanDecode() gopacket.LayerClass {
	return LayerTypeLLC
}

// NextLayerType returns the layer type contained by this DecodingLayer.
func (l *LLC) NextLayerType() gopacket.LayerType {
	switch {
	case l.DSAP == 0xAA && l.SSAP == 0xAA:
		return LayerTypeSNAP
	case l.DSAP == 0x42 && l.SSAP == 0x42:
		return LayerTypeSTP
	}
	return gopacket.LayerTypeZero // Not implemented
}

// SNAP is used inside LLC.  See
// http://standards.ieee.org/getieee802/download/802-2001.pdf.
// From http://en.wikipedia.org/wiki/Subnetwork_Access_Protocol:
//  "[T]he Subnetwork Access Protocol (SNAP) is a mechanism for multiplexing,
//  on networks using IEEE 802.2 LLC, more protocols than can be distinguished
//  by the 8-bit 802.2 Service Access Point (SAP) fields."
type SNAP struct {
	BaseLayer
	OrganizationalCode []byte
	Type               EthernetType
}

// LayerType returns gopacket.LayerTypeSNAP.
func (s *SNAP) LayerType() gopacket.LayerType { return LayerTypeSNAP }

// DecodeFromBytes decodes the given bytes into this layer.
func (s *SNAP) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 5 {
		return errors.New("SNAP header too small")
	}
	s.OrganizationalCode = data[:3]
	s.Type = EthernetType(binary.BigEndian.Uint16(data[3:5]))
	s.BaseLayer = BaseLayer{data[:5], data[5:]}
	return nil
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (s *SNAP) CanDecode() gopacket.LayerClass {
	return LayerTypeSNAP
}

// NextLayerType returns the layer type contained by this DecodingLayer.
func (s *SNAP) NextLayerType() gopacket.LayerType {
	// See BUG(gconnel) in decodeSNAP
	return s.Type.LayerType()
}

func decodeLLC(data []byte, p gopacket.PacketBuilder) error {
	l := &LLC{}
	err := l.DecodeFromBytes(data, p)
	if err != nil {
		return err
	}
	p.AddLayer(l)
	return p.NextDecoder(l.NextLayerType())
}

func decodeSNAP(data []byte, p gopacket.PacketBuilder) error {
	s := &SNAP{}
	err := s.DecodeFromBytes(data, p)
	if err != nil {
		return err
	}
	p.AddLayer(s)
	// BUG(gconnell):  When decoding SNAP, we treat the SNAP type as an Ethernet
	// type.  This may not actually be an ethernet type in all cases,
	// depending on the organizational code.  Right now, we don't check.
	return p.NextDecoder(s.Type)
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (l *LLC) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	var igFlag, crFlag byte
	var length int

	if l.Control&0xFF00 != 0 {
		length = 4
	} else {
		length = 3
	}

	if l.DSAP&0x1 != 0 {
		return errors.New("DSAP value invalid, should not include IG flag bit")
	}

	if l.SSAP&0x1 != 0 {
		return errors.New("SSAP value invalid, should not include CR flag bit")
	}

	if buf, err := b.PrependBytes(length); err != nil {
		return err
	} else {
		igFlag = 0
		if l.IG {
			igFlag = 0x1
		}

		crFlag = 0
		if l.CR {
			crFlag = 0x1
		}

		buf[0] = l.DSAP + igFlag
		buf[1] = l.SSAP + crFlag

		if length == 4 {
			buf[2] = uint8(l.Control >> 8)
			buf[3] = uint8(l.Control)
		} else {
			buf[2] = uint8(l.Control)
		}
	}

	return nil
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (s *SNAP) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	if buf, err := b.PrependBytes(5); err != nil {
		return err
	} else {
		buf[0] = s.OrganizationalCode[0]
		buf[1] = s.OrganizationalCode[1]
		buf[2] = s.OrganizationalCode[2]
		binary.BigEndian.PutUint16(buf[3:5], uint16(s.Type))
	}

	return nil
}
