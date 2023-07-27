// Copyright 2012 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package gopacket

import (
	"errors"
)

// DecodeFeedback is used by DecodingLayer layers to provide decoding metadata.
type DecodeFeedback interface {
	// SetTruncated should be called if during decoding you notice that a packet
	// is shorter than internal layer variables (HeaderLength, or the like) say it
	// should be.  It sets packet.Metadata().Truncated.
	SetTruncated()
}

type nilDecodeFeedback struct{}

func (nilDecodeFeedback) SetTruncated() {}

// NilDecodeFeedback implements DecodeFeedback by doing nothing.
var NilDecodeFeedback DecodeFeedback = nilDecodeFeedback{}

// PacketBuilder is used by layer decoders to store the layers they've decoded,
// and to defer future decoding via NextDecoder.
// Typically, the pattern for use is:
//  func (m *myDecoder) Decode(data []byte, p PacketBuilder) error {
//    if myLayer, err := myDecodingLogic(data); err != nil {
//      return err
//    } else {
//      p.AddLayer(myLayer)
//    }
//    // maybe do this, if myLayer is a LinkLayer
//    p.SetLinkLayer(myLayer)
//    return p.NextDecoder(nextDecoder)
//  }
type PacketBuilder interface {
	DecodeFeedback
	// AddLayer should be called by a decoder immediately upon successful
	// decoding of a layer.
	AddLayer(l Layer)
	// The following functions set the various specific layers in the final
	// packet.  Note that if many layers call SetX, the first call is kept and all
	// other calls are ignored.
	SetLinkLayer(LinkLayer)
	SetNetworkLayer(NetworkLayer)
	SetTransportLayer(TransportLayer)
	SetApplicationLayer(ApplicationLayer)
	SetErrorLayer(ErrorLayer)
	// NextDecoder should be called by a decoder when they're done decoding a
	// packet layer but not done with decoding the entire packet.  The next
	// decoder will be called to decode the last AddLayer's LayerPayload.
	// Because of this, NextDecoder must only be called once all other
	// PacketBuilder calls have been made.  Set*Layer and AddLayer calls after
	// NextDecoder calls will behave incorrectly.
	NextDecoder(next Decoder) error
	// DumpPacketData is used solely for decoding.  If you come across an error
	// you need to diagnose while processing a packet, call this and your packet's
	// data will be dumped to stderr so you can create a test.  This should never
	// be called from a production decoder.
	DumpPacketData()
	// DecodeOptions returns the decode options
	DecodeOptions() *DecodeOptions
}

// Decoder is an interface for logic to decode a packet layer.  Users may
// implement a Decoder to handle their own strange packet types, or may use one
// of the many decoders available in the 'layers' subpackage to decode things
// for them.
type Decoder interface {
	// Decode decodes the bytes of a packet, sending decoded values and other
	// information to PacketBuilder, and returning an error if unsuccessful.  See
	// the PacketBuilder documentation for more details.
	Decode([]byte, PacketBuilder) error
}

// DecodeFunc wraps a function to make it a Decoder.
type DecodeFunc func([]byte, PacketBuilder) error

// Decode implements Decoder by calling itself.
func (d DecodeFunc) Decode(data []byte, p PacketBuilder) error {
	// function, call thyself.
	return d(data, p)
}

// DecodePayload is a Decoder that returns a Payload layer containing all
// remaining bytes.
var DecodePayload Decoder = DecodeFunc(decodePayload)

// DecodeUnknown is a Decoder that returns an Unknown layer containing all
// remaining bytes, useful if you run up against a layer that you're unable to
// decode yet.  This layer is considered an ErrorLayer.
var DecodeUnknown Decoder = DecodeFunc(decodeUnknown)

// DecodeFragment is a Decoder that returns a Fragment layer containing all
// remaining bytes.
var DecodeFragment Decoder = DecodeFunc(decodeFragment)

// LayerTypeZero is an invalid layer type, but can be used to determine whether
// layer type has actually been set correctly.
var LayerTypeZero = RegisterLayerType(0, LayerTypeMetadata{Name: "Unknown", Decoder: DecodeUnknown})

// LayerTypeDecodeFailure is the layer type for the default error layer.
var LayerTypeDecodeFailure = RegisterLayerType(1, LayerTypeMetadata{Name: "DecodeFailure", Decoder: DecodeUnknown})

// LayerTypePayload is the layer type for a payload that we don't try to decode
// but treat as a success, IE: an application-level payload.
var LayerTypePayload = RegisterLayerType(2, LayerTypeMetadata{Name: "Payload", Decoder: DecodePayload})

// LayerTypeFragment is the layer type for a fragment of a layer transported
// by an underlying layer that supports fragmentation.
var LayerTypeFragment = RegisterLayerType(3, LayerTypeMetadata{Name: "Fragment", Decoder: DecodeFragment})

// DecodeFailure is a packet layer created if decoding of the packet data failed
// for some reason.  It implements ErrorLayer.  LayerContents will be the entire
// set of bytes that failed to parse, and Error will return the reason parsing
// failed.
type DecodeFailure struct {
	data  []byte
	err   error
	stack []byte
}

// Error returns the error encountered during decoding.
func (d *DecodeFailure) Error() error { return d.err }

// LayerContents implements Layer.
func (d *DecodeFailure) LayerContents() []byte { return d.data }

// LayerPayload implements Layer.
func (d *DecodeFailure) LayerPayload() []byte { return nil }

// String implements fmt.Stringer.
func (d *DecodeFailure) String() string {
	return "Packet decoding error: " + d.Error().Error()
}

// Dump implements Dumper.
func (d *DecodeFailure) Dump() (s string) {
	if d.stack != nil {
		s = string(d.stack)
	}
	return
}

// LayerType returns LayerTypeDecodeFailure
func (d *DecodeFailure) LayerType() LayerType { return LayerTypeDecodeFailure }

// decodeUnknown "decodes" unsupported data types by returning an error.
// This decoder will thus always return a DecodeFailure layer.
func decodeUnknown(data []byte, p PacketBuilder) error {
	return errors.New("Layer type not currently supported")
}
