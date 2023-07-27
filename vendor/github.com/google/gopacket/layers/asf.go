// Copyright 2019 The GoPacket Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file in the root of the source tree.

package layers

// This file implements the ASF RMCP payload specified in section 3.2.2.3 of
// https://www.dmtf.org/sites/default/files/standards/documents/DSP0136.pdf

import (
	"encoding/binary"
	"fmt"

	"github.com/google/gopacket"
)

const (
	// ASFRMCPEnterprise is the IANA-assigned Enterprise Number of the ASF-RMCP.
	ASFRMCPEnterprise uint32 = 4542
)

// ASFDataIdentifier encapsulates fields used to uniquely identify the format of
// the data block.
//
// While the enterprise number is almost always 4542 (ASF-RMCP), we support
// registering layers using structs of this type as a key in case any users are
// using OEM-extensions.
type ASFDataIdentifier struct {

	// Enterprise is the IANA Enterprise Number associated with the entity that
	// defines the message type. A list can be found at
	// https://www.iana.org/assignments/enterprise-numbers/enterprise-numbers.
	// This can be thought of as the namespace for the message type.
	Enterprise uint32

	// Type is the message type, defined by the entity associated with the
	// enterprise above. No pressure, but in the context of EN 4542, 1 byte is
	// the difference between sending a ping and telling a machine to do an
	// unconditional power down (0x80 and 0x12 respectively).
	Type uint8
}

// LayerType returns the payload layer type corresponding to an ASF message
// type.
func (a ASFDataIdentifier) LayerType() gopacket.LayerType {
	if lt := asfDataLayerTypes[a]; lt != 0 {
		return lt
	}

	// some layer types don't have a payload, e.g. ASF-RMCP Presence Ping.
	return gopacket.LayerTypePayload
}

// RegisterASFLayerType allows specifying that the data block of ASF packets
// with a given enterprise number and type should be processed by a given layer
// type. This overrides any existing registrations, including defaults.
func RegisterASFLayerType(a ASFDataIdentifier, l gopacket.LayerType) {
	asfDataLayerTypes[a] = l
}

var (
	// ASFDataIdentifierPresencePong is the message type of the response to a
	// Presence Ping message. It indicates the sender is ASF-RMCP-aware.
	ASFDataIdentifierPresencePong = ASFDataIdentifier{
		Enterprise: ASFRMCPEnterprise,
		Type:       0x40,
	}

	// ASFDataIdentifierPresencePing is a message type sent to a managed client
	// to solicit a Presence Pong response. Clients may ignore this if the RMCP
	// version is unsupported. Sending this message with a sequence number <255
	// is the recommended way of finding out whether an implementation sends
	// RMCP ACKs (e.g. iDRAC does, Super Micro does not).
	//
	// Systems implementing IPMI must respond to this ping to conform to the
	// spec, so it is a good substitute for an ICMP ping.
	ASFDataIdentifierPresencePing = ASFDataIdentifier{
		Enterprise: ASFRMCPEnterprise,
		Type:       0x80,
	}

	// asfDataLayerTypes is used to find the next layer for a given ASF header.
	asfDataLayerTypes = map[ASFDataIdentifier]gopacket.LayerType{
		ASFDataIdentifierPresencePong: LayerTypeASFPresencePong,
	}
)

// ASF defines ASF's generic RMCP message Data block format. See section
// 3.2.2.3.
type ASF struct {
	BaseLayer
	ASFDataIdentifier

	// Tag is used to match request/response pairs. The tag of a response is set
	// to that of the message it is responding to. If a message is
	// unidirectional, i.e. not part of a request/response pair, this is set to
	// 255.
	Tag uint8

	// 1 byte reserved, set to 0x00.

	// Length is the length of this layer's payload in bytes.
	Length uint8
}

// LayerType returns LayerTypeASF. It partially satisfies Layer and
// SerializableLayer.
func (*ASF) LayerType() gopacket.LayerType {
	return LayerTypeASF
}

// CanDecode returns LayerTypeASF. It partially satisfies DecodingLayer.
func (a *ASF) CanDecode() gopacket.LayerClass {
	return a.LayerType()
}

// DecodeFromBytes makes the layer represent the provided bytes. It partially
// satisfies DecodingLayer.
func (a *ASF) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 8 {
		df.SetTruncated()
		return fmt.Errorf("invalid ASF data header, length %v less than 8",
			len(data))
	}

	a.BaseLayer.Contents = data[:8]
	a.BaseLayer.Payload = data[8:]

	a.Enterprise = binary.BigEndian.Uint32(data[:4])
	a.Type = uint8(data[4])
	a.Tag = uint8(data[5])
	// 1 byte reserved
	a.Length = uint8(data[7])
	return nil
}

// NextLayerType returns the layer type corresponding to the message type of
// this ASF data layer. This partially satisfies DecodingLayer.
func (a *ASF) NextLayerType() gopacket.LayerType {
	return a.ASFDataIdentifier.LayerType()
}

// SerializeTo writes the serialized fom of this layer into the SerializeBuffer,
// partially satisfying SerializableLayer.
func (a *ASF) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	payload := b.Bytes()
	bytes, err := b.PrependBytes(8)
	if err != nil {
		return err
	}
	binary.BigEndian.PutUint32(bytes[:4], a.Enterprise)
	bytes[4] = uint8(a.Type)
	bytes[5] = a.Tag
	bytes[6] = 0x00
	if opts.FixLengths {
		a.Length = uint8(len(payload))
	}
	bytes[7] = a.Length
	return nil
}

// decodeASF decodes the byte slice into an RMCP-ASF data struct.
func decodeASF(data []byte, p gopacket.PacketBuilder) error {
	return decodingLayerDecoder(&ASF{}, data, p)
}
