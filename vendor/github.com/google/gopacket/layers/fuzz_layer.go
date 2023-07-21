// Copyright 2019 The GoPacket Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file in the root of the source tree.

package layers

import (
	"encoding/binary"

	"github.com/google/gopacket"
)

// FuzzLayer is a fuzz target for the layers package of gopacket
// A fuzz target is a function processing a binary blob (byte slice)
// The process here is to interpret this data as a packet, and print the layers contents.
// The decoding options and the starting layer are encoded in the first bytes.
// The function returns 1 if this is a valid packet (no error layer)
func FuzzLayer(data []byte) int {
	if len(data) < 3 {
		return 0
	}
	// use the first two bytes to choose the top level layer
	startLayer := binary.BigEndian.Uint16(data[:2])
	var fuzzOpts = gopacket.DecodeOptions{
		Lazy:                     data[2]&0x1 != 0,
		NoCopy:                   data[2]&0x2 != 0,
		SkipDecodeRecovery:       data[2]&0x4 != 0,
		DecodeStreamsAsDatagrams: data[2]&0x8 != 0,
	}
	p := gopacket.NewPacket(data[3:], gopacket.LayerType(startLayer), fuzzOpts)
	for _, l := range p.Layers() {
		gopacket.LayerString(l)
	}
	if p.ErrorLayer() != nil {
		return 0
	}
	return 1
}
