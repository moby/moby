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

// PPPoE is the layer for PPPoE encapsulation headers.
type PPPoE struct {
	BaseLayer
	Version   uint8
	Type      uint8
	Code      PPPoECode
	SessionId uint16
	Length    uint16
}

// LayerType returns gopacket.LayerTypePPPoE.
func (p *PPPoE) LayerType() gopacket.LayerType {
	return LayerTypePPPoE
}

// decodePPPoE decodes the PPPoE header (see http://tools.ietf.org/html/rfc2516).
func decodePPPoE(data []byte, p gopacket.PacketBuilder) error {
	pppoe := &PPPoE{
		Version:   data[0] >> 4,
		Type:      data[0] & 0x0F,
		Code:      PPPoECode(data[1]),
		SessionId: binary.BigEndian.Uint16(data[2:4]),
		Length:    binary.BigEndian.Uint16(data[4:6]),
	}
	pppoe.BaseLayer = BaseLayer{data[:6], data[6 : 6+pppoe.Length]}
	p.AddLayer(pppoe)
	return p.NextDecoder(pppoe.Code)
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (p *PPPoE) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	payload := b.Bytes()
	bytes, err := b.PrependBytes(6)
	if err != nil {
		return err
	}
	bytes[0] = (p.Version << 4) | p.Type
	bytes[1] = byte(p.Code)
	binary.BigEndian.PutUint16(bytes[2:], p.SessionId)
	if opts.FixLengths {
		p.Length = uint16(len(payload))
	}
	binary.BigEndian.PutUint16(bytes[4:], p.Length)
	return nil
}
