// Copyright 2012 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"github.com/google/gopacket"
)

// BaseLayer is a convenience struct which implements the LayerData and
// LayerPayload functions of the Layer interface.
type BaseLayer struct {
	// Contents is the set of bytes that make up this layer.  IE: for an
	// Ethernet packet, this would be the set of bytes making up the
	// Ethernet frame.
	Contents []byte
	// Payload is the set of bytes contained by (but not part of) this
	// Layer.  Again, to take Ethernet as an example, this would be the
	// set of bytes encapsulated by the Ethernet protocol.
	Payload []byte
}

// LayerContents returns the bytes of the packet layer.
func (b *BaseLayer) LayerContents() []byte { return b.Contents }

// LayerPayload returns the bytes contained within the packet layer.
func (b *BaseLayer) LayerPayload() []byte { return b.Payload }

type layerDecodingLayer interface {
	gopacket.Layer
	DecodeFromBytes([]byte, gopacket.DecodeFeedback) error
	NextLayerType() gopacket.LayerType
}

func decodingLayerDecoder(d layerDecodingLayer, data []byte, p gopacket.PacketBuilder) error {
	err := d.DecodeFromBytes(data, p)
	if err != nil {
		return err
	}
	p.AddLayer(d)
	next := d.NextLayerType()
	if next == gopacket.LayerTypeZero {
		return nil
	}
	return p.NextDecoder(next)
}

// hacky way to zero out memory... there must be a better way?
var lotsOfZeros [1024]byte
