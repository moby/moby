// Copyright 2012 Google, Inc. All rights reserved.
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

type RUDP struct {
	BaseLayer
	SYN, ACK, EACK, RST, NUL bool
	Version                  uint8
	HeaderLength             uint8
	SrcPort, DstPort         RUDPPort
	DataLength               uint16
	Seq, Ack, Checksum       uint32
	VariableHeaderArea       []byte
	// RUDPHeaderSyn contains SYN information for the RUDP packet,
	// if the SYN flag is set
	*RUDPHeaderSYN
	// RUDPHeaderEack contains EACK information for the RUDP packet,
	// if the EACK flag is set.
	*RUDPHeaderEACK
}

type RUDPHeaderSYN struct {
	MaxOutstandingSegments, MaxSegmentSize, OptionFlags uint16
}

type RUDPHeaderEACK struct {
	SeqsReceivedOK []uint32
}

// LayerType returns gopacket.LayerTypeRUDP.
func (r *RUDP) LayerType() gopacket.LayerType { return LayerTypeRUDP }

func decodeRUDP(data []byte, p gopacket.PacketBuilder) error {
	r := &RUDP{
		SYN:          data[0]&0x80 != 0,
		ACK:          data[0]&0x40 != 0,
		EACK:         data[0]&0x20 != 0,
		RST:          data[0]&0x10 != 0,
		NUL:          data[0]&0x08 != 0,
		Version:      data[0] & 0x3,
		HeaderLength: data[1],
		SrcPort:      RUDPPort(data[2]),
		DstPort:      RUDPPort(data[3]),
		DataLength:   binary.BigEndian.Uint16(data[4:6]),
		Seq:          binary.BigEndian.Uint32(data[6:10]),
		Ack:          binary.BigEndian.Uint32(data[10:14]),
		Checksum:     binary.BigEndian.Uint32(data[14:18]),
	}
	if r.HeaderLength < 9 {
		return fmt.Errorf("RUDP packet with too-short header length %d", r.HeaderLength)
	}
	hlen := int(r.HeaderLength) * 2
	r.Contents = data[:hlen]
	r.Payload = data[hlen : hlen+int(r.DataLength)]
	r.VariableHeaderArea = data[18:hlen]
	headerData := r.VariableHeaderArea
	switch {
	case r.SYN:
		if len(headerData) != 6 {
			return fmt.Errorf("RUDP packet invalid SYN header length: %d", len(headerData))
		}
		r.RUDPHeaderSYN = &RUDPHeaderSYN{
			MaxOutstandingSegments: binary.BigEndian.Uint16(headerData[:2]),
			MaxSegmentSize:         binary.BigEndian.Uint16(headerData[2:4]),
			OptionFlags:            binary.BigEndian.Uint16(headerData[4:6]),
		}
	case r.EACK:
		if len(headerData)%4 != 0 {
			return fmt.Errorf("RUDP packet invalid EACK header length: %d", len(headerData))
		}
		r.RUDPHeaderEACK = &RUDPHeaderEACK{make([]uint32, len(headerData)/4)}
		for i := 0; i < len(headerData); i += 4 {
			r.SeqsReceivedOK[i/4] = binary.BigEndian.Uint32(headerData[i : i+4])
		}
	}
	p.AddLayer(r)
	p.SetTransportLayer(r)
	return p.NextDecoder(gopacket.LayerTypePayload)
}

func (r *RUDP) TransportFlow() gopacket.Flow {
	return gopacket.NewFlow(EndpointRUDPPort, []byte{byte(r.SrcPort)}, []byte{byte(r.DstPort)})
}
