// Copyright 2012 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package gopacket

import (
	"fmt"
)

// Layer represents a single decoded packet layer (using either the
// OSI or TCP/IP definition of a layer).  When decoding, a packet's data is
// broken up into a number of layers.  The caller may call LayerType() to
// figure out which type of layer they've received from the packet.  Optionally,
// they may then use a type assertion to get the actual layer type for deep
// inspection of the data.
type Layer interface {
	// LayerType is the gopacket type for this layer.
	LayerType() LayerType
	// LayerContents returns the set of bytes that make up this layer.
	LayerContents() []byte
	// LayerPayload returns the set of bytes contained within this layer, not
	// including the layer itself.
	LayerPayload() []byte
}

// Payload is a Layer containing the payload of a packet.  The definition of
// what constitutes the payload of a packet depends on previous layers; for
// TCP and UDP, we stop decoding above layer 4 and return the remaining
// bytes as a Payload.  Payload is an ApplicationLayer.
type Payload []byte

// LayerType returns LayerTypePayload
func (p Payload) LayerType() LayerType { return LayerTypePayload }

// LayerContents returns the bytes making up this layer.
func (p Payload) LayerContents() []byte { return []byte(p) }

// LayerPayload returns the payload within this layer.
func (p Payload) LayerPayload() []byte { return nil }

// Payload returns this layer as bytes.
func (p Payload) Payload() []byte { return []byte(p) }

// String implements fmt.Stringer.
func (p Payload) String() string { return fmt.Sprintf("%d byte(s)", len(p)) }

// GoString implements fmt.GoStringer.
func (p Payload) GoString() string { return LongBytesGoString([]byte(p)) }

// CanDecode implements DecodingLayer.
func (p Payload) CanDecode() LayerClass { return LayerTypePayload }

// NextLayerType implements DecodingLayer.
func (p Payload) NextLayerType() LayerType { return LayerTypeZero }

// DecodeFromBytes implements DecodingLayer.
func (p *Payload) DecodeFromBytes(data []byte, df DecodeFeedback) error {
	*p = Payload(data)
	return nil
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (p Payload) SerializeTo(b SerializeBuffer, opts SerializeOptions) error {
	bytes, err := b.PrependBytes(len(p))
	if err != nil {
		return err
	}
	copy(bytes, p)
	return nil
}

// decodePayload decodes data by returning it all in a Payload layer.
func decodePayload(data []byte, p PacketBuilder) error {
	payload := &Payload{}
	if err := payload.DecodeFromBytes(data, p); err != nil {
		return err
	}
	p.AddLayer(payload)
	p.SetApplicationLayer(payload)
	return nil
}

// Fragment is a Layer containing a fragment of a larger frame, used by layers
// like IPv4 and IPv6 that allow for fragmentation of their payloads.
type Fragment []byte

// LayerType returns LayerTypeFragment
func (p *Fragment) LayerType() LayerType { return LayerTypeFragment }

// LayerContents implements Layer.
func (p *Fragment) LayerContents() []byte { return []byte(*p) }

// LayerPayload implements Layer.
func (p *Fragment) LayerPayload() []byte { return nil }

// Payload returns this layer as a byte slice.
func (p *Fragment) Payload() []byte { return []byte(*p) }

// String implements fmt.Stringer.
func (p *Fragment) String() string { return fmt.Sprintf("%d byte(s)", len(*p)) }

// CanDecode implements DecodingLayer.
func (p *Fragment) CanDecode() LayerClass { return LayerTypeFragment }

// NextLayerType implements DecodingLayer.
func (p *Fragment) NextLayerType() LayerType { return LayerTypeZero }

// DecodeFromBytes implements DecodingLayer.
func (p *Fragment) DecodeFromBytes(data []byte, df DecodeFeedback) error {
	*p = Fragment(data)
	return nil
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (p *Fragment) SerializeTo(b SerializeBuffer, opts SerializeOptions) error {
	bytes, err := b.PrependBytes(len(*p))
	if err != nil {
		return err
	}
	copy(bytes, *p)
	return nil
}

// decodeFragment decodes data by returning it all in a Fragment layer.
func decodeFragment(data []byte, p PacketBuilder) error {
	payload := &Fragment{}
	if err := payload.DecodeFromBytes(data, p); err != nil {
		return err
	}
	p.AddLayer(payload)
	p.SetApplicationLayer(payload)
	return nil
}

// These layers correspond to Internet Protocol Suite (TCP/IP) layers, and their
// corresponding OSI layers, as best as possible.

// LinkLayer is the packet layer corresponding to TCP/IP layer 1 (OSI layer 2)
type LinkLayer interface {
	Layer
	LinkFlow() Flow
}

// NetworkLayer is the packet layer corresponding to TCP/IP layer 2 (OSI
// layer 3)
type NetworkLayer interface {
	Layer
	NetworkFlow() Flow
}

// TransportLayer is the packet layer corresponding to the TCP/IP layer 3 (OSI
// layer 4)
type TransportLayer interface {
	Layer
	TransportFlow() Flow
}

// ApplicationLayer is the packet layer corresponding to the TCP/IP layer 4 (OSI
// layer 7), also known as the packet payload.
type ApplicationLayer interface {
	Layer
	Payload() []byte
}

// ErrorLayer is a packet layer created when decoding of the packet has failed.
// Its payload is all the bytes that we were unable to decode, and the returned
// error details why the decoding failed.
type ErrorLayer interface {
	Layer
	Error() error
}
