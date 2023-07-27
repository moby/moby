// Copyright 2017 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.
//

package layers

import (
	"encoding/binary"
	"fmt"
	"github.com/google/gopacket"
)

const gtpMinimumSizeInBytes int = 8

// GTPExtensionHeader is used to carry extra data and enable future extensions of the GTP  without the need to use another version number.
type GTPExtensionHeader struct {
	Type    uint8
	Content []byte
}

// GTPv1U protocol is used to exchange user data over GTP tunnels across the Sx interfaces.
// Defined in https://portal.3gpp.org/desktopmodules/Specifications/SpecificationDetails.aspx?specificationId=1595
type GTPv1U struct {
	BaseLayer
	Version             uint8
	ProtocolType        uint8
	Reserved            uint8
	ExtensionHeaderFlag bool
	SequenceNumberFlag  bool
	NPDUFlag            bool
	MessageType         uint8
	MessageLength       uint16
	TEID                uint32
	SequenceNumber      uint16
	NPDU                uint8
	GTPExtensionHeaders []GTPExtensionHeader
}

// LayerType returns LayerTypeGTPV1U
func (g *GTPv1U) LayerType() gopacket.LayerType { return LayerTypeGTPv1U }

// DecodeFromBytes analyses a byte slice and attempts to decode it as a GTPv1U packet
func (g *GTPv1U) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	hLen := gtpMinimumSizeInBytes
	dLen := len(data)
	if dLen < hLen {
		return fmt.Errorf("GTP packet too small: %d bytes", dLen)
	}
	g.Version = (data[0] >> 5) & 0x07
	g.ProtocolType = (data[0] >> 4) & 0x01
	g.Reserved = (data[0] >> 3) & 0x01
	g.SequenceNumberFlag = ((data[0] >> 1) & 0x01) == 1
	g.NPDUFlag = (data[0] & 0x01) == 1
	g.ExtensionHeaderFlag = ((data[0] >> 2) & 0x01) == 1
	g.MessageType = data[1]
	g.MessageLength = binary.BigEndian.Uint16(data[2:4])
	pLen := 8 + g.MessageLength
	if uint16(dLen) < pLen {
		return fmt.Errorf("GTP packet too small: %d bytes", dLen)
	}
	//  Field used to multiplex different connections in the same GTP tunnel.
	g.TEID = binary.BigEndian.Uint32(data[4:8])
	cIndex := uint16(hLen)
	if g.SequenceNumberFlag || g.NPDUFlag || g.ExtensionHeaderFlag {
		hLen += 4
		cIndex += 4
		if dLen < hLen {
			return fmt.Errorf("GTP packet too small: %d bytes", dLen)
		}
		if g.SequenceNumberFlag {
			g.SequenceNumber = binary.BigEndian.Uint16(data[8:10])
		}
		if g.NPDUFlag {
			g.NPDU = data[10]
		}
		if g.ExtensionHeaderFlag {
			extensionFlag := true
			for extensionFlag {
				extensionType := uint8(data[cIndex-1])
				extensionLength := uint(data[cIndex])
				if extensionLength == 0 {
					return fmt.Errorf("GTP packet with invalid extension header")
				}
				// extensionLength is in 4-octet units
				lIndex := cIndex + (uint16(extensionLength) * 4)
				if uint16(dLen) < lIndex {
					fmt.Println(dLen, lIndex)
					return fmt.Errorf("GTP packet with small extension header: %d bytes", dLen)
				}
				content := data[cIndex+1 : lIndex-1]
				eh := GTPExtensionHeader{Type: extensionType, Content: content}
				g.GTPExtensionHeaders = append(g.GTPExtensionHeaders, eh)
				cIndex = lIndex
				// Check if coming bytes are from an extension header
				extensionFlag = data[cIndex-1] != 0

			}
		}
	}
	g.BaseLayer = BaseLayer{Contents: data[:cIndex], Payload: data[cIndex:]}
	return nil

}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (g *GTPv1U) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	data, err := b.PrependBytes(gtpMinimumSizeInBytes)
	if err != nil {
		return err
	}
	data[0] |= (g.Version << 5)
	data[0] |= (1 << 4)
	if len(g.GTPExtensionHeaders) > 0 {
		data[0] |= 0x04
		g.ExtensionHeaderFlag = true
	}
	if g.SequenceNumberFlag {
		data[0] |= 0x02
	}
	if g.NPDUFlag {
		data[0] |= 0x01
	}
	data[1] = g.MessageType
	binary.BigEndian.PutUint16(data[2:4], g.MessageLength)
	binary.BigEndian.PutUint32(data[4:8], g.TEID)
	if g.ExtensionHeaderFlag || g.SequenceNumberFlag || g.NPDUFlag {
		data, err := b.AppendBytes(4)
		if err != nil {
			return err
		}
		binary.BigEndian.PutUint16(data[:2], g.SequenceNumber)
		data[2] = g.NPDU
		for _, eh := range g.GTPExtensionHeaders {
			data[len(data)-1] = eh.Type
			lContent := len(eh.Content)
			// extensionLength is in 4-octet units
			extensionLength := (lContent + 2) / 4
			// Get two extra byte for the next extension header type and length
			data, err = b.AppendBytes(lContent + 2)
			if err != nil {
				return err
			}
			data[0] = byte(extensionLength)
			copy(data[1:lContent+1], eh.Content)
		}
	}
	return nil

}

// CanDecode returns a set of layers that GTP objects can decode.
func (g *GTPv1U) CanDecode() gopacket.LayerClass {
	return LayerTypeGTPv1U
}

// NextLayerType specifies the next layer that GoPacket should attempt to
func (g *GTPv1U) NextLayerType() gopacket.LayerType {
	if len(g.LayerPayload()) == 0 {
		return gopacket.LayerTypeZero
	}
	version := uint8(g.LayerPayload()[0]) >> 4
	if version == 4 {
		return LayerTypeIPv4
	} else if version == 6 {
		return LayerTypeIPv6
	} else {
		return LayerTypePPP
	}
}

func decodeGTPv1u(data []byte, p gopacket.PacketBuilder) error {
	gtp := &GTPv1U{}
	err := gtp.DecodeFromBytes(data, p)
	if err != nil {
		return err
	}
	p.AddLayer(gtp)
	return p.NextDecoder(gtp.NextLayerType())
}
