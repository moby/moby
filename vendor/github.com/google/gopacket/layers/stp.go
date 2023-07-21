// Copyright 2017 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"github.com/google/gopacket"
)

// STP decode spanning tree protocol packets to transport BPDU (bridge protocol data unit) message.
type STP struct {
	BaseLayer
}

// LayerType returns gopacket.LayerTypeSTP.
func (s *STP) LayerType() gopacket.LayerType { return LayerTypeSTP }

func decodeSTP(data []byte, p gopacket.PacketBuilder) error {
	stp := &STP{}
	stp.Contents = data[:]
	// TODO:  parse the STP protocol into actual subfields.
	p.AddLayer(stp)
	return nil
}
