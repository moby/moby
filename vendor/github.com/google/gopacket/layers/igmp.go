// Copyright 2012 Google, Inc. All rights reserved.
// Copyright 2009-2011 Andreas Krennmair. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"encoding/binary"
	"errors"
	"net"
	"time"

	"github.com/google/gopacket"
)

type IGMPType uint8

const (
	IGMPMembershipQuery    IGMPType = 0x11 // General or group specific query
	IGMPMembershipReportV1 IGMPType = 0x12 // Version 1 Membership Report
	IGMPMembershipReportV2 IGMPType = 0x16 // Version 2 Membership Report
	IGMPLeaveGroup         IGMPType = 0x17 // Leave Group
	IGMPMembershipReportV3 IGMPType = 0x22 // Version 3 Membership Report
)

// String conversions for IGMP message types
func (i IGMPType) String() string {
	switch i {
	case IGMPMembershipQuery:
		return "IGMP Membership Query"
	case IGMPMembershipReportV1:
		return "IGMPv1 Membership Report"
	case IGMPMembershipReportV2:
		return "IGMPv2 Membership Report"
	case IGMPMembershipReportV3:
		return "IGMPv3 Membership Report"
	case IGMPLeaveGroup:
		return "Leave Group"
	default:
		return ""
	}
}

type IGMPv3GroupRecordType uint8

const (
	IGMPIsIn  IGMPv3GroupRecordType = 0x01 // Type MODE_IS_INCLUDE, source addresses x
	IGMPIsEx  IGMPv3GroupRecordType = 0x02 // Type MODE_IS_EXCLUDE, source addresses x
	IGMPToIn  IGMPv3GroupRecordType = 0x03 // Type CHANGE_TO_INCLUDE_MODE, source addresses x
	IGMPToEx  IGMPv3GroupRecordType = 0x04 // Type CHANGE_TO_EXCLUDE_MODE, source addresses x
	IGMPAllow IGMPv3GroupRecordType = 0x05 // Type ALLOW_NEW_SOURCES, source addresses x
	IGMPBlock IGMPv3GroupRecordType = 0x06 // Type BLOCK_OLD_SOURCES, source addresses x
)

func (i IGMPv3GroupRecordType) String() string {
	switch i {
	case IGMPIsIn:
		return "MODE_IS_INCLUDE"
	case IGMPIsEx:
		return "MODE_IS_EXCLUDE"
	case IGMPToIn:
		return "CHANGE_TO_INCLUDE_MODE"
	case IGMPToEx:
		return "CHANGE_TO_EXCLUDE_MODE"
	case IGMPAllow:
		return "ALLOW_NEW_SOURCES"
	case IGMPBlock:
		return "BLOCK_OLD_SOURCES"
	default:
		return ""
	}
}

// IGMP represents an IGMPv3 message.
type IGMP struct {
	BaseLayer
	Type                    IGMPType
	MaxResponseTime         time.Duration
	Checksum                uint16
	GroupAddress            net.IP
	SupressRouterProcessing bool
	RobustnessValue         uint8
	IntervalTime            time.Duration
	SourceAddresses         []net.IP
	NumberOfGroupRecords    uint16
	NumberOfSources         uint16
	GroupRecords            []IGMPv3GroupRecord
	Version                 uint8 // IGMP protocol version
}

// IGMPv1or2 stores header details for an IGMPv1 or IGMPv2 packet.
//
//  0                   1                   2                   3
//  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |      Type     | Max Resp Time |           Checksum            |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                         Group Address                         |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
type IGMPv1or2 struct {
	BaseLayer
	Type            IGMPType      // IGMP message type
	MaxResponseTime time.Duration // meaningful only in Membership Query messages
	Checksum        uint16        // 16-bit checksum of entire ip payload
	GroupAddress    net.IP        // either 0 or an IP multicast address
	Version         uint8
}

// decodeResponse dissects IGMPv1 or IGMPv2 packet.
func (i *IGMPv1or2) decodeResponse(data []byte) error {
	if len(data) < 8 {
		return errors.New("IGMP packet too small")
	}

	i.MaxResponseTime = igmpTimeDecode(data[1])
	i.Checksum = binary.BigEndian.Uint16(data[2:4])
	i.GroupAddress = net.IP(data[4:8])

	return nil
}

//  0                   1                   2                   3
//  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |  Type = 0x22  |    Reserved   |           Checksum            |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |           Reserved            |  Number of Group Records (M)  |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                                                               |
// .                        Group Record [1]                       .
// |                                                               |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                                                               |
// .                        Group Record [2]                       .
// |                                                               |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                                                               |
// .                        Group Record [M]                       .
// |                                                               |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |  Record Type  |  Aux Data Len |     Number of Sources (N)     |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                       Multicast Address                       |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                       Source Address [1]                      |
// +-                                                             -+
// |                       Source Address [2]                      |
// +-                                                             -+
// |                       Source Address [N]                      |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                                                               |
// .                         Auxiliary Data                        .
// |                                                               |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

// IGMPv3GroupRecord stores individual group records for a V3 Membership Report message.
type IGMPv3GroupRecord struct {
	Type             IGMPv3GroupRecordType
	AuxDataLen       uint8 // this should always be 0 as per IGMPv3 spec.
	NumberOfSources  uint16
	MulticastAddress net.IP
	SourceAddresses  []net.IP
	AuxData          uint32 // NOT USED
}

func (i *IGMP) decodeIGMPv3MembershipReport(data []byte) error {
	if len(data) < 8 {
		return errors.New("IGMPv3 Membership Report too small #1")
	}

	i.Checksum = binary.BigEndian.Uint16(data[2:4])
	i.NumberOfGroupRecords = binary.BigEndian.Uint16(data[6:8])

	recordOffset := 8
	for j := 0; j < int(i.NumberOfGroupRecords); j++ {
		if len(data) < recordOffset+8 {
			return errors.New("IGMPv3 Membership Report too small #2")
		}

		var gr IGMPv3GroupRecord
		gr.Type = IGMPv3GroupRecordType(data[recordOffset])
		gr.AuxDataLen = data[recordOffset+1]
		gr.NumberOfSources = binary.BigEndian.Uint16(data[recordOffset+2 : recordOffset+4])
		gr.MulticastAddress = net.IP(data[recordOffset+4 : recordOffset+8])

		if len(data) < recordOffset+8+int(gr.NumberOfSources)*4 {
			return errors.New("IGMPv3 Membership Report too small #3")
		}

		// append source address records.
		for i := 0; i < int(gr.NumberOfSources); i++ {
			sourceAddr := net.IP(data[recordOffset+8+i*4 : recordOffset+12+i*4])
			gr.SourceAddresses = append(gr.SourceAddresses, sourceAddr)
		}

		i.GroupRecords = append(i.GroupRecords, gr)
		recordOffset += 8 + 4*int(gr.NumberOfSources)
	}
	return nil
}

//  0                   1                   2                   3
//  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |  Type = 0x11  | Max Resp Code |           Checksum            |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                         Group Address                         |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// | Resv  |S| QRV |     QQIC      |     Number of Sources (N)     |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                       Source Address [1]                      |
// +-                                                             -+
// |                       Source Address [2]                      |
// +-                              .                              -+
// |                       Source Address [N]                      |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// decodeIGMPv3MembershipQuery parses the IGMPv3 message of type 0x11
func (i *IGMP) decodeIGMPv3MembershipQuery(data []byte) error {
	if len(data) < 12 {
		return errors.New("IGMPv3 Membership Query too small #1")
	}

	i.MaxResponseTime = igmpTimeDecode(data[1])
	i.Checksum = binary.BigEndian.Uint16(data[2:4])
	i.SupressRouterProcessing = data[8]&0x8 != 0
	i.GroupAddress = net.IP(data[4:8])
	i.RobustnessValue = data[8] & 0x7
	i.IntervalTime = igmpTimeDecode(data[9])
	i.NumberOfSources = binary.BigEndian.Uint16(data[10:12])

	if len(data) < 12+int(i.NumberOfSources)*4 {
		return errors.New("IGMPv3 Membership Query too small #2")
	}

	for j := 0; j < int(i.NumberOfSources); j++ {
		i.SourceAddresses = append(i.SourceAddresses, net.IP(data[12+j*4:16+j*4]))
	}

	return nil
}

// igmpTimeDecode decodes the duration created by the given byte, using the
// algorithm in http://www.rfc-base.org/txt/rfc-3376.txt section 4.1.1.
func igmpTimeDecode(t uint8) time.Duration {
	if t&0x80 == 0 {
		return time.Millisecond * 100 * time.Duration(t)
	}
	mant := (t & 0x70) >> 4
	exp := t & 0x0F
	return time.Millisecond * 100 * time.Duration((mant|0x10)<<(exp+3))
}

// LayerType returns LayerTypeIGMP for the V1,2,3 message protocol formats.
func (i *IGMP) LayerType() gopacket.LayerType      { return LayerTypeIGMP }
func (i *IGMPv1or2) LayerType() gopacket.LayerType { return LayerTypeIGMP }

func (i *IGMPv1or2) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 8 {
		return errors.New("IGMP Packet too small")
	}

	i.Type = IGMPType(data[0])
	i.MaxResponseTime = igmpTimeDecode(data[1])
	i.Checksum = binary.BigEndian.Uint16(data[2:4])
	i.GroupAddress = net.IP(data[4:8])

	return nil
}

func (i *IGMPv1or2) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypeZero
}

func (i *IGMPv1or2) CanDecode() gopacket.LayerClass {
	return LayerTypeIGMP
}

// DecodeFromBytes decodes the given bytes into this layer.
func (i *IGMP) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 1 {
		return errors.New("IGMP packet is too small")
	}

	// common IGMP header values between versions 1..3 of IGMP specification..
	i.Type = IGMPType(data[0])

	switch i.Type {
	case IGMPMembershipQuery:
		i.decodeIGMPv3MembershipQuery(data)
	case IGMPMembershipReportV3:
		i.decodeIGMPv3MembershipReport(data)
	default:
		return errors.New("unsupported IGMP type")
	}

	return nil
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (i *IGMP) CanDecode() gopacket.LayerClass {
	return LayerTypeIGMP
}

// NextLayerType returns the layer type contained by this DecodingLayer.
func (i *IGMP) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypeZero
}

// decodeIGMP will parse IGMP v1,2 or 3 protocols. Checks against the
// IGMP type are performed against byte[0], logic then iniitalizes and
// passes the appropriate struct (IGMP or IGMPv1or2) to
// decodingLayerDecoder.
func decodeIGMP(data []byte, p gopacket.PacketBuilder) error {
	if len(data) < 1 {
		return errors.New("IGMP packet is too small")
	}

	// byte 0 contains IGMP message type.
	switch IGMPType(data[0]) {
	case IGMPMembershipQuery:
		// IGMPv3 Membership Query payload is >= 12
		if len(data) >= 12 {
			i := &IGMP{Version: 3}
			return decodingLayerDecoder(i, data, p)
		} else if len(data) == 8 {
			i := &IGMPv1or2{}
			if data[1] == 0x00 {
				i.Version = 1 // IGMPv1 has a query length of 8 and MaxResp = 0
			} else {
				i.Version = 2 // IGMPv2 has a query length of 8 and MaxResp != 0
			}

			return decodingLayerDecoder(i, data, p)
		}
	case IGMPMembershipReportV3:
		i := &IGMP{Version: 3}
		return decodingLayerDecoder(i, data, p)
	case IGMPMembershipReportV1:
		i := &IGMPv1or2{Version: 1}
		return decodingLayerDecoder(i, data, p)
	case IGMPLeaveGroup, IGMPMembershipReportV2:
		// leave group and Query Report v2 used in IGMPv2 only.
		i := &IGMPv1or2{Version: 2}
		return decodingLayerDecoder(i, data, p)
	default:
	}

	return errors.New("Unable to determine IGMP type.")
}
