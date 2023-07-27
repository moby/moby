// Copyright 2012 Google, Inc. All rights reserved.
// Copyright 2009-2011 Andreas Krennmair. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"encoding/binary"
	"github.com/google/gopacket"
)

// UDPLite is the layer for UDP-Lite headers (rfc 3828).
type UDPLite struct {
	BaseLayer
	SrcPort, DstPort UDPLitePort
	ChecksumCoverage uint16
	Checksum         uint16
	sPort, dPort     []byte
}

// LayerType returns gopacket.LayerTypeUDPLite
func (u *UDPLite) LayerType() gopacket.LayerType { return LayerTypeUDPLite }

func decodeUDPLite(data []byte, p gopacket.PacketBuilder) error {
	udp := &UDPLite{
		SrcPort:          UDPLitePort(binary.BigEndian.Uint16(data[0:2])),
		sPort:            data[0:2],
		DstPort:          UDPLitePort(binary.BigEndian.Uint16(data[2:4])),
		dPort:            data[2:4],
		ChecksumCoverage: binary.BigEndian.Uint16(data[4:6]),
		Checksum:         binary.BigEndian.Uint16(data[6:8]),
		BaseLayer:        BaseLayer{data[:8], data[8:]},
	}
	p.AddLayer(udp)
	p.SetTransportLayer(udp)
	return p.NextDecoder(gopacket.LayerTypePayload)
}

func (u *UDPLite) TransportFlow() gopacket.Flow {
	return gopacket.NewFlow(EndpointUDPLitePort, u.sPort, u.dPort)
}
