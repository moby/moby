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

// EtherIP is the struct for storing RFC 3378 EtherIP packet headers.
type EtherIP struct {
	BaseLayer
	Version  uint8
	Reserved uint16
}

// LayerType returns gopacket.LayerTypeEtherIP.
func (e *EtherIP) LayerType() gopacket.LayerType { return LayerTypeEtherIP }

// DecodeFromBytes decodes the given bytes into this layer.
func (e *EtherIP) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	e.Version = data[0] >> 4
	e.Reserved = binary.BigEndian.Uint16(data[:2]) & 0x0fff
	e.BaseLayer = BaseLayer{data[:2], data[2:]}
	return nil
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (e *EtherIP) CanDecode() gopacket.LayerClass {
	return LayerTypeEtherIP
}

// NextLayerType returns the layer type contained by this DecodingLayer.
func (e *EtherIP) NextLayerType() gopacket.LayerType {
	return LayerTypeEthernet
}

func decodeEtherIP(data []byte, p gopacket.PacketBuilder) error {
	e := &EtherIP{}
	return decodingLayerDecoder(e, data, p)
}
