// Copyright 2018 Google, Inc. All rights reserved.
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

const (
	// LCMShortHeaderMagic is the LCM small message header magic number
	LCMShortHeaderMagic uint32 = 0x4c433032
	// LCMFragmentedHeaderMagic is the LCM fragmented message header magic number
	LCMFragmentedHeaderMagic uint32 = 0x4c433033
)

// LCM (Lightweight Communications and Marshalling) is a set of libraries and
// tools for message passing and data marshalling, targeted at real-time systems
// where high-bandwidth and low latency are critical. It provides a
// publish/subscribe message passing model and automatic
// marshalling/unmarshalling code generation with bindings for applications in a
// variety of programming languages.
//
// References
//   https://lcm-proj.github.io/
//   https://github.com/lcm-proj/lcm
type LCM struct {
	// Common (short & fragmented header) fields
	Magic          uint32
	SequenceNumber uint32
	// Fragmented header only fields
	PayloadSize    uint32
	FragmentOffset uint32
	FragmentNumber uint16
	TotalFragments uint16
	// Common field
	ChannelName string
	// Gopacket helper fields
	Fragmented  bool
	fingerprint LCMFingerprint
	contents    []byte
	payload     []byte
}

// LCMFingerprint is the type of a LCM fingerprint.
type LCMFingerprint uint64

var (
	// lcmLayerTypes contains a map of all LCM fingerprints that we support and
	// their LayerType
	lcmLayerTypes  = map[LCMFingerprint]gopacket.LayerType{}
	layerTypeIndex = 1001
)

// RegisterLCMLayerType allows users to register decoders for the underlying
// LCM payload. This is done based on the fingerprint that every LCM message
// contains and which identifies it uniquely. If num is not the zero value it
// will be used when registering with RegisterLayerType towards gopacket,
// otherwise an incremental value starting from 1001 will be used.
func RegisterLCMLayerType(num int, name string, fingerprint LCMFingerprint,
	decoder gopacket.Decoder) gopacket.LayerType {
	metadata := gopacket.LayerTypeMetadata{Name: name, Decoder: decoder}

	if num == 0 {
		num = layerTypeIndex
		layerTypeIndex++
	}

	lcmLayerTypes[fingerprint] = gopacket.RegisterLayerType(num, metadata)

	return lcmLayerTypes[fingerprint]
}

// SupportedLCMFingerprints returns a slice of all LCM fingerprints that has
// been registered so far.
func SupportedLCMFingerprints() []LCMFingerprint {
	fingerprints := make([]LCMFingerprint, 0, len(lcmLayerTypes))
	for fp := range lcmLayerTypes {
		fingerprints = append(fingerprints, fp)
	}
	return fingerprints
}

// GetLCMLayerType returns the underlying LCM message's LayerType.
// This LayerType has to be registered by using RegisterLCMLayerType.
func GetLCMLayerType(fingerprint LCMFingerprint) gopacket.LayerType {
	layerType, ok := lcmLayerTypes[fingerprint]
	if !ok {
		return gopacket.LayerTypePayload
	}

	return layerType
}

func decodeLCM(data []byte, p gopacket.PacketBuilder) error {
	lcm := &LCM{}

	err := lcm.DecodeFromBytes(data, p)
	if err != nil {
		return err
	}

	p.AddLayer(lcm)
	p.SetApplicationLayer(lcm)

	return p.NextDecoder(lcm.NextLayerType())
}

// DecodeFromBytes decodes the given bytes into this layer.
func (lcm *LCM) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 8 {
		df.SetTruncated()
		return errors.New("LCM < 8 bytes")
	}
	offset := 0

	lcm.Magic = binary.BigEndian.Uint32(data[offset:4])
	offset += 4

	if lcm.Magic != LCMShortHeaderMagic && lcm.Magic != LCMFragmentedHeaderMagic {
		return fmt.Errorf("Received LCM header magic %v does not match know "+
			"LCM magic numbers. Dropping packet.", lcm.Magic)
	}

	lcm.SequenceNumber = binary.BigEndian.Uint32(data[offset:8])
	offset += 4

	if lcm.Magic == LCMFragmentedHeaderMagic {
		lcm.Fragmented = true

		lcm.PayloadSize = binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		lcm.FragmentOffset = binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		lcm.FragmentNumber = binary.BigEndian.Uint16(data[offset : offset+2])
		offset += 2

		lcm.TotalFragments = binary.BigEndian.Uint16(data[offset : offset+2])
		offset += 2
	} else {
		lcm.Fragmented = false
	}

	if !lcm.Fragmented || (lcm.Fragmented && lcm.FragmentNumber == 0) {
		buffer := make([]byte, 0)
		for _, b := range data[offset:] {
			offset++

			if b == 0 {
				break
			}

			buffer = append(buffer, b)
		}

		lcm.ChannelName = string(buffer)
	}

	lcm.fingerprint = LCMFingerprint(
		binary.BigEndian.Uint64(data[offset : offset+8]))

	lcm.contents = data[:offset]
	lcm.payload = data[offset:]

	return nil
}

// CanDecode returns a set of layers that LCM objects can decode.
// As LCM objects can only decode the LCM layer, we just return that layer.
func (lcm LCM) CanDecode() gopacket.LayerClass {
	return LayerTypeLCM
}

// NextLayerType specifies the LCM payload layer type following this header.
// As LCM packets are serialized structs with uniq fingerprints for each uniq
// combination of data types, lookup of correct layer type is based on that
// fingerprint.
func (lcm LCM) NextLayerType() gopacket.LayerType {
	if !lcm.Fragmented || (lcm.Fragmented && lcm.FragmentNumber == 0) {
		return GetLCMLayerType(lcm.fingerprint)
	}

	return gopacket.LayerTypeFragment
}

// LayerType returns LayerTypeLCM
func (lcm LCM) LayerType() gopacket.LayerType {
	return LayerTypeLCM
}

// LayerContents returns the contents of the LCM header.
func (lcm LCM) LayerContents() []byte {
	return lcm.contents
}

// LayerPayload returns the payload following this LCM header.
func (lcm LCM) LayerPayload() []byte {
	return lcm.payload
}

// Payload returns the payload following this LCM header.
func (lcm LCM) Payload() []byte {
	return lcm.LayerPayload()
}

// Fingerprint returns the LCM fingerprint of the underlying message.
func (lcm LCM) Fingerprint() LCMFingerprint {
	return lcm.fingerprint
}
