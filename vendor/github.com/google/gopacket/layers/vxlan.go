// Copyright 2016 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/google/gopacket"
)

//  VXLAN is specifed in RFC 7348 https://tools.ietf.org/html/rfc7348
//  G, D, A, Group Policy ID from https://tools.ietf.org/html/draft-smith-vxlan-group-policy-00
//  0                   1                   2                   3
//  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//  0             8               16              24              32
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |G|R|R|R|I|R|R|R|R|D|R|R|A|R|R|R|       Group Policy ID         |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |     24 bit VXLAN Network Identifier           |   Reserved    |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

// VXLAN is a VXLAN packet header
type VXLAN struct {
	BaseLayer
	ValidIDFlag      bool   // 'I' bit per RFC 7348
	VNI              uint32 // 'VXLAN Network Identifier' 24 bits per RFC 7348
	GBPExtension     bool   // 'G' bit per Group Policy https://tools.ietf.org/html/draft-smith-vxlan-group-policy-00
	GBPDontLearn     bool   // 'D' bit per Group Policy
	GBPApplied       bool   // 'A' bit per Group Policy
	GBPGroupPolicyID uint16 // 'Group Policy ID' 16 bits per Group Policy
}

// LayerType returns LayerTypeVXLAN
func (vx *VXLAN) LayerType() gopacket.LayerType { return LayerTypeVXLAN }

// CanDecode returns the layer type this DecodingLayer can decode
func (vx *VXLAN) CanDecode() gopacket.LayerClass {
	return LayerTypeVXLAN
}

// NextLayerType retuns the next layer we should see after vxlan
func (vx *VXLAN) NextLayerType() gopacket.LayerType {
	return LayerTypeEthernet
}

// DecodeFromBytes takes a byte buffer and decodes
func (vx *VXLAN) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 8 {
		return errors.New("vxlan packet too small")
	}
	// VNI is a 24bit number, Uint32 requires 32 bits
	var buf [4]byte
	copy(buf[1:], data[4:7])

	// RFC 7348 https://tools.ietf.org/html/rfc7348
	vx.ValidIDFlag = data[0]&0x08 > 0        // 'I' bit per RFC7348
	vx.VNI = binary.BigEndian.Uint32(buf[:]) // VXLAN Network Identifier per RFC7348

	// Group Based Policy https://tools.ietf.org/html/draft-smith-vxlan-group-policy-00
	vx.GBPExtension = data[0]&0x80 > 0                       // 'G' bit per the group policy draft
	vx.GBPDontLearn = data[1]&0x40 > 0                       // 'D' bit - the egress VTEP MUST NOT learn the source address of the encapsulated frame.
	vx.GBPApplied = data[1]&0x80 > 0                         // 'A' bit - indicates that the group policy has already been applied to this packet.
	vx.GBPGroupPolicyID = binary.BigEndian.Uint16(data[2:4]) // Policy ID as per the group policy draft

	// Layer information
	const vxlanLength = 8
	vx.Contents = data[:vxlanLength]
	vx.Payload = data[vxlanLength:]

	return nil

}

func decodeVXLAN(data []byte, p gopacket.PacketBuilder) error {
	vx := &VXLAN{}
	err := vx.DecodeFromBytes(data, p)
	if err != nil {
		return err
	}

	p.AddLayer(vx)
	return p.NextDecoder(LinkTypeEthernet)
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (vx *VXLAN) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	bytes, err := b.PrependBytes(8)
	if err != nil {
		return err
	}

	// PrependBytes does not guarantee that bytes are zeroed.  Setting flags via OR requires that they start off at zero
	bytes[0] = 0
	bytes[1] = 0

	if vx.ValidIDFlag {
		bytes[0] |= 0x08
	}
	if vx.GBPExtension {
		bytes[0] |= 0x80
	}
	if vx.GBPDontLearn {
		bytes[1] |= 0x40
	}
	if vx.GBPApplied {
		bytes[1] |= 0x80
	}

	binary.BigEndian.PutUint16(bytes[2:4], vx.GBPGroupPolicyID)
	if vx.VNI >= 1<<24 {
		return fmt.Errorf("Virtual Network Identifier = %x exceeds max for 24-bit uint", vx.VNI)
	}
	binary.BigEndian.PutUint32(bytes[4:8], vx.VNI<<8)
	return nil
}
