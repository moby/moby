// Copyright 2012 Google, Inc. All rights reserved.
// Copyright 2009-2011 Andreas Krennmair. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"encoding/binary"
	"fmt"

	"github.com/google/gopacket"
)

// UDP is the layer for UDP headers.
type UDP struct {
	BaseLayer
	SrcPort, DstPort UDPPort
	Length           uint16
	Checksum         uint16
	sPort, dPort     []byte
	tcpipchecksum
}

// LayerType returns gopacket.LayerTypeUDP
func (u *UDP) LayerType() gopacket.LayerType { return LayerTypeUDP }

func (udp *UDP) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 8 {
		df.SetTruncated()
		return fmt.Errorf("Invalid UDP header. Length %d less than 8", len(data))
	}
	udp.SrcPort = UDPPort(binary.BigEndian.Uint16(data[0:2]))
	udp.sPort = data[0:2]
	udp.DstPort = UDPPort(binary.BigEndian.Uint16(data[2:4]))
	udp.dPort = data[2:4]
	udp.Length = binary.BigEndian.Uint16(data[4:6])
	udp.Checksum = binary.BigEndian.Uint16(data[6:8])
	udp.BaseLayer = BaseLayer{Contents: data[:8]}
	switch {
	case udp.Length >= 8:
		hlen := int(udp.Length)
		if hlen > len(data) {
			df.SetTruncated()
			hlen = len(data)
		}
		udp.Payload = data[8:hlen]
	case udp.Length == 0: // Jumbogram, use entire rest of data
		udp.Payload = data[8:]
	default:
		return fmt.Errorf("UDP packet too small: %d bytes", udp.Length)
	}
	return nil
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (u *UDP) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	var jumbo bool

	payload := b.Bytes()
	if _, ok := u.pseudoheader.(*IPv6); ok {
		if len(payload)+8 > 65535 {
			jumbo = true
		}
	}
	bytes, err := b.PrependBytes(8)
	if err != nil {
		return err
	}
	binary.BigEndian.PutUint16(bytes, uint16(u.SrcPort))
	binary.BigEndian.PutUint16(bytes[2:], uint16(u.DstPort))
	if opts.FixLengths {
		if jumbo {
			u.Length = 0
		} else {
			u.Length = uint16(len(payload)) + 8
		}
	}
	binary.BigEndian.PutUint16(bytes[4:], u.Length)
	if opts.ComputeChecksums {
		// zero out checksum bytes
		bytes[6] = 0
		bytes[7] = 0
		csum, err := u.computeChecksum(b.Bytes(), IPProtocolUDP)
		if err != nil {
			return err
		}
		u.Checksum = csum
	}
	binary.BigEndian.PutUint16(bytes[6:], u.Checksum)
	return nil
}

func (u *UDP) CanDecode() gopacket.LayerClass {
	return LayerTypeUDP
}

// NextLayerType use the destination port to select the
// right next decoder. It tries first to decode via the
// destination port, then the source port.
func (u *UDP) NextLayerType() gopacket.LayerType {
	if lt := u.DstPort.LayerType(); lt != gopacket.LayerTypePayload {
		return lt
	}
	return u.SrcPort.LayerType()
}

func decodeUDP(data []byte, p gopacket.PacketBuilder) error {
	udp := &UDP{}
	err := udp.DecodeFromBytes(data, p)
	p.AddLayer(udp)
	p.SetTransportLayer(udp)
	if err != nil {
		return err
	}
	return p.NextDecoder(udp.NextLayerType())
}

func (u *UDP) TransportFlow() gopacket.Flow {
	return gopacket.NewFlow(EndpointUDPPort, u.sPort, u.dPort)
}

// For testing only
func (u *UDP) SetInternalPortsForTesting() {
	u.sPort = make([]byte, 2)
	u.dPort = make([]byte, 2)
	binary.BigEndian.PutUint16(u.sPort, uint16(u.SrcPort))
	binary.BigEndian.PutUint16(u.dPort, uint16(u.DstPort))
}
