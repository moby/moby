// Copyright 2019 The GoPacket Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file in the root of the source tree.

package layers

// This file implements the ASF-RMCP header specified in section 3.2.2.2 of
// https://www.dmtf.org/sites/default/files/standards/documents/DSP0136.pdf

import (
	"fmt"

	"github.com/google/gopacket"
)

// RMCPClass is the class of a RMCP layer's payload, e.g. ASF or IPMI. This is a
// 4-bit unsigned int on the wire; all but 6 (ASF), 7 (IPMI) and 8 (OEM-defined)
// are currently reserved.
type RMCPClass uint8

// LayerType returns the payload layer type corresponding to a RMCP class.
func (c RMCPClass) LayerType() gopacket.LayerType {
	if lt := rmcpClassLayerTypes[uint8(c)]; lt != 0 {
		return lt
	}
	return gopacket.LayerTypePayload
}

func (c RMCPClass) String() string {
	return fmt.Sprintf("%v(%v)", uint8(c), c.LayerType())
}

const (
	// RMCPVersion1 identifies RMCP v1.0 in the Version header field. Lower
	// values are considered legacy, while higher values are reserved by the
	// specification.
	RMCPVersion1 uint8 = 0x06

	// RMCPNormal indicates a "normal" message, i.e. not an acknowledgement.
	RMCPNormal uint8 = 0

	// RMCPAck indicates a message is acknowledging a received normal message.
	RMCPAck uint8 = 1 << 7

	// RMCPClassASF identifies an RMCP message as containing an ASF-RMCP
	// payload.
	RMCPClassASF RMCPClass = 0x06

	// RMCPClassIPMI identifies an RMCP message as containing an IPMI payload.
	RMCPClassIPMI RMCPClass = 0x07

	// RMCPClassOEM identifies an RMCP message as containing an OEM-defined
	// payload.
	RMCPClassOEM RMCPClass = 0x08
)

var (
	rmcpClassLayerTypes = [16]gopacket.LayerType{
		RMCPClassASF: LayerTypeASF,
		// RMCPClassIPMI is to implement; RMCPClassOEM is deliberately not
		// implemented, so we return LayerTypePayload
	}
)

// RegisterRMCPLayerType allows specifying that the payload of a RMCP packet of
// a certain class should processed by the provided layer type. This overrides
// any existing registrations, including defaults.
func RegisterRMCPLayerType(c RMCPClass, l gopacket.LayerType) {
	rmcpClassLayerTypes[c] = l
}

// RMCP describes the format of an RMCP header, which forms a UDP payload. See
// section 3.2.2.2.
type RMCP struct {
	BaseLayer

	// Version identifies the version of the RMCP header. 0x06 indicates RMCP
	// v1.0; lower values are legacy, higher values are reserved.
	Version uint8

	// Sequence is the sequence number assicated with the message. Note that
	// this rolls over to 0 after 254, not 255. Seq num 255 indicates the
	// receiver must not send an ACK.
	Sequence uint8

	// Ack indicates whether this packet is an acknowledgement. If it is, the
	// payload will be empty.
	Ack bool

	// Class idicates the structure of the payload. There are only 2^4 valid
	// values, however there is no uint4 data type. N.B. the Ack bit has been
	// split off into another field. The most significant 4 bits of this field
	// will always be 0.
	Class RMCPClass
}

// LayerType returns LayerTypeRMCP. It partially satisfies Layer and
// SerializableLayer.
func (*RMCP) LayerType() gopacket.LayerType {
	return LayerTypeRMCP
}

// CanDecode returns LayerTypeRMCP. It partially satisfies DecodingLayer.
func (r *RMCP) CanDecode() gopacket.LayerClass {
	return r.LayerType()
}

// DecodeFromBytes makes the layer represent the provided bytes. It partially
// satisfies DecodingLayer.
func (r *RMCP) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 4 {
		df.SetTruncated()
		return fmt.Errorf("invalid RMCP header, length %v less than 4",
			len(data))
	}

	r.BaseLayer.Contents = data[:4]
	r.BaseLayer.Payload = data[4:]

	r.Version = uint8(data[0])
	// 1 byte reserved
	r.Sequence = uint8(data[2])
	r.Ack = data[3]&RMCPAck != 0
	r.Class = RMCPClass(data[3] & 0xF)
	return nil
}

// NextLayerType returns the data layer of this RMCP layer. This partially
// satisfies DecodingLayer.
func (r *RMCP) NextLayerType() gopacket.LayerType {
	return r.Class.LayerType()
}

// Payload returns the data layer. It partially satisfies ApplicationLayer.
func (r *RMCP) Payload() []byte {
	return r.BaseLayer.Payload
}

// SerializeTo writes the serialized fom of this layer into the SerializeBuffer,
// partially satisfying SerializableLayer.
func (r *RMCP) SerializeTo(b gopacket.SerializeBuffer, _ gopacket.SerializeOptions) error {
	// The IPMI v1.5 spec contains a pad byte for frame sizes of certain lengths
	// to work around issues in LAN chips. This is no longer necessary as of
	// IPMI v2.0 (renamed to "legacy pad") so we do not attempt to add it. The
	// same approach is taken by FreeIPMI:
	// http://git.savannah.gnu.org/cgit/freeipmi.git/tree/libfreeipmi/interface/ipmi-lan-interface.c?id=b5ffcd38317daf42074458879f4c55ba6804a595#n836
	bytes, err := b.PrependBytes(4)
	if err != nil {
		return err
	}
	bytes[0] = r.Version
	bytes[1] = 0x00
	bytes[2] = r.Sequence
	bytes[3] = bool2uint8(r.Ack)<<7 | uint8(r.Class) // thanks, BFD layer
	return nil
}

// decodeRMCP decodes the byte slice into an RMCP type, and sets the application
// layer to it.
func decodeRMCP(data []byte, p gopacket.PacketBuilder) error {
	rmcp := &RMCP{}
	err := rmcp.DecodeFromBytes(data, p)
	p.AddLayer(rmcp)
	p.SetApplicationLayer(rmcp)
	if err != nil {
		return err
	}
	return p.NextDecoder(rmcp.NextLayerType())
}
