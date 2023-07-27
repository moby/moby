// Copyright 2012 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"github.com/google/gopacket"
	"net"
)

// FDDI contains the header for FDDI frames.
type FDDI struct {
	BaseLayer
	FrameControl   FDDIFrameControl
	Priority       uint8
	SrcMAC, DstMAC net.HardwareAddr
}

// LayerType returns LayerTypeFDDI.
func (f *FDDI) LayerType() gopacket.LayerType { return LayerTypeFDDI }

// LinkFlow returns a new flow of type EndpointMAC.
func (f *FDDI) LinkFlow() gopacket.Flow {
	return gopacket.NewFlow(EndpointMAC, f.SrcMAC, f.DstMAC)
}

func decodeFDDI(data []byte, p gopacket.PacketBuilder) error {
	f := &FDDI{
		FrameControl: FDDIFrameControl(data[0] & 0xF8),
		Priority:     data[0] & 0x07,
		SrcMAC:       net.HardwareAddr(data[1:7]),
		DstMAC:       net.HardwareAddr(data[7:13]),
		BaseLayer:    BaseLayer{data[:13], data[13:]},
	}
	p.SetLinkLayer(f)
	p.AddLayer(f)
	return p.NextDecoder(f.FrameControl)
}
