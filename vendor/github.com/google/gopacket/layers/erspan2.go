// Copyright 2018 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"encoding/binary"

	"github.com/google/gopacket"
)

const (
	//ERSPANIIVersionObsolete - The obsolete value for the version field
	ERSPANIIVersionObsolete = 0x0
	// ERSPANIIVersion - The current value for the version field
	ERSPANIIVersion = 0x1
)

// ERSPANII contains all of the fields found in an ERSPAN Type II header
// https://tools.ietf.org/html/draft-foschiano-erspan-03
type ERSPANII struct {
	BaseLayer
	IsTruncated                         bool
	Version, CoS, TrunkEncap            uint8
	VLANIdentifier, SessionID, Reserved uint16
	Index                               uint32
}

func (erspan2 *ERSPANII) LayerType() gopacket.LayerType { return LayerTypeERSPANII }

// DecodeFromBytes decodes the given bytes into this layer.
func (erspan2 *ERSPANII) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	erspan2Length := 8
	erspan2.Version = data[0] & 0xF0 >> 4
	erspan2.VLANIdentifier = binary.BigEndian.Uint16(data[:2]) & 0x0FFF
	erspan2.CoS = data[2] & 0xE0 >> 5
	erspan2.TrunkEncap = data[2] & 0x18 >> 3
	erspan2.IsTruncated = data[2]&0x4>>2 != 0
	erspan2.SessionID = binary.BigEndian.Uint16(data[2:4]) & 0x03FF
	erspan2.Reserved = binary.BigEndian.Uint16(data[4:6]) & 0xFFF0 >> 4
	erspan2.Index = binary.BigEndian.Uint32(data[4:8]) & 0x000FFFFF
	erspan2.Contents = data[:erspan2Length]
	erspan2.Payload = data[erspan2Length:]
	return nil
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (erspan2 *ERSPANII) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	bytes, err := b.PrependBytes(8)
	if err != nil {
		return err
	}

	twoByteInt := uint16(erspan2.Version&0xF)<<12 | erspan2.VLANIdentifier&0x0FFF
	binary.BigEndian.PutUint16(bytes, twoByteInt)

	twoByteInt = uint16(erspan2.CoS&0x7)<<13 | uint16(erspan2.TrunkEncap&0x3)<<11 | erspan2.SessionID&0x03FF
	if erspan2.IsTruncated {
		twoByteInt |= 0x400
	}
	binary.BigEndian.PutUint16(bytes[2:], twoByteInt)

	fourByteInt := uint32(erspan2.Reserved&0x0FFF)<<20 | erspan2.Index&0x000FFFFF
	binary.BigEndian.PutUint32(bytes[4:], fourByteInt)
	return nil
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (erspan2 *ERSPANII) CanDecode() gopacket.LayerClass {
	return LayerTypeERSPANII
}

// NextLayerType returns the layer type contained by this DecodingLayer.
func (erspan2 *ERSPANII) NextLayerType() gopacket.LayerType {
	return LayerTypeEthernet
}

func decodeERSPANII(data []byte, p gopacket.PacketBuilder) error {
	erspan2 := &ERSPANII{}
	return decodingLayerDecoder(erspan2, data, p)
}
