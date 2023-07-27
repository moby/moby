// Copyright 2014 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

/*
This layer decodes SFlow version 5 datagrams.

The specification can be found here: http://sflow.org/sflow_version_5.txt

Additional developer information about sflow can be found at:
http://sflow.org/developers/specifications.php

And SFlow in general:
http://sflow.org/index.php

Two forms of sample data are defined: compact and expanded. The
Specification has this to say:

    Compact and expand forms of counter and flow samples are defined.
    An agent must not mix compact/expanded encodings.  If an agent
    will never use ifIndex numbers >= 2^24 then it must use compact
    encodings for all interfaces.  Otherwise the expanded formats must
    be used for all interfaces.

This decoder only supports the compact form, because that is the only
one for which data was available.

The datagram is composed of one or more samples of type flow or counter,
and each sample is composed of one or more records describing the sample.
A sample is a single instance of sampled inforamtion, and each record in
the sample gives additional / supplimentary information about the sample.

The following sample record types are supported:

	Raw Packet Header
	opaque = flow_data; enterprise = 0; format = 1

	Extended Switch Data
	opaque = flow_data; enterprise = 0; format = 1001

	Extended Router Data
	opaque = flow_data; enterprise = 0; format = 1002

	Extended Gateway Data
	opaque = flow_data; enterprise = 0; format = 1003

	Extended User Data
	opaque = flow_data; enterprise = 0; format = 1004

	Extended URL Data
	opaque = flow_data; enterprise = 0; format = 1005

The following types of counter records are supported:

	Generic Interface Counters - see RFC 2233
	opaque = counter_data; enterprise = 0; format = 1

	Ethernet Interface Counters - see RFC 2358
	opaque = counter_data; enterprise = 0; format = 2

SFlow is encoded using XDR (RFC4506). There are a few places
where the standard 4-byte fields are partitioned into two
bitfields of different lengths. I'm not sure why the designers
chose to pack together two values like this in some places, and
in others they use the entire 4-byte value to store a number that
will never be more than a few bits. In any case, there are a couple
of types defined to handle the decoding of these bitfields, and
that's why they're there. */

package layers

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"

	"github.com/google/gopacket"
)

// SFlowRecord holds both flow sample records and counter sample records.
// A Record is the structure that actually holds the sampled data
// and / or counters.
type SFlowRecord interface {
}

// SFlowDataSource encodes a 2-bit SFlowSourceFormat in its most significant
// 2 bits, and an SFlowSourceValue in its least significant 30 bits.
// These types and values define the meaning of the inteface information
// presented in the sample metadata.
type SFlowDataSource int32

func (sdc SFlowDataSource) decode() (SFlowSourceFormat, SFlowSourceValue) {
	leftField := sdc >> 30
	rightField := uint32(0x3FFFFFFF) & uint32(sdc)
	return SFlowSourceFormat(leftField), SFlowSourceValue(rightField)
}

type SFlowDataSourceExpanded struct {
	SourceIDClass SFlowSourceFormat
	SourceIDIndex SFlowSourceValue
}

func (sdce SFlowDataSourceExpanded) decode() (SFlowSourceFormat, SFlowSourceValue) {
	leftField := sdce.SourceIDClass >> 30
	rightField := uint32(0x3FFFFFFF) & uint32(sdce.SourceIDIndex)
	return SFlowSourceFormat(leftField), SFlowSourceValue(rightField)
}

type SFlowSourceFormat uint32

type SFlowSourceValue uint32

const (
	SFlowTypeSingleInterface      SFlowSourceFormat = 0
	SFlowTypePacketDiscarded      SFlowSourceFormat = 1
	SFlowTypeMultipleDestinations SFlowSourceFormat = 2
)

func (sdf SFlowSourceFormat) String() string {
	switch sdf {
	case SFlowTypeSingleInterface:
		return "Single Interface"
	case SFlowTypePacketDiscarded:
		return "Packet Discarded"
	case SFlowTypeMultipleDestinations:
		return "Multiple Destinations"
	default:
		return "UNKNOWN"
	}
}

func decodeSFlow(data []byte, p gopacket.PacketBuilder) error {
	s := &SFlowDatagram{}
	err := s.DecodeFromBytes(data, p)
	if err != nil {
		return err
	}
	p.AddLayer(s)
	p.SetApplicationLayer(s)
	return nil
}

// SFlowDatagram is the outermost container which holds some basic information
// about the reporting agent, and holds at least one sample record
type SFlowDatagram struct {
	BaseLayer

	DatagramVersion uint32
	AgentAddress    net.IP
	SubAgentID      uint32
	SequenceNumber  uint32
	AgentUptime     uint32
	SampleCount     uint32
	FlowSamples     []SFlowFlowSample
	CounterSamples  []SFlowCounterSample
}

// An SFlow  datagram's outer container has the following
// structure:

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |           int sFlow version (2|4|5)           |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |   int IP version of the Agent (1=v4|2=v6)     |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  /    Agent IP address (v4=4byte|v6=16byte)      /
//  /                                               /
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |               int sub agent id                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |         int datagram sequence number          |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |            int switch uptime in ms            |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |          int n samples in datagram            |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  /                  n samples                    /
//  /                                               /
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

// SFlowDataFormat encodes the EnterpriseID in the most
// significant 12 bits, and the SampleType in the least significant
// 20 bits.
type SFlowDataFormat uint32

func (sdf SFlowDataFormat) decode() (SFlowEnterpriseID, SFlowSampleType) {
	leftField := sdf >> 12
	rightField := uint32(0xFFF) & uint32(sdf)
	return SFlowEnterpriseID(leftField), SFlowSampleType(rightField)
}

// SFlowEnterpriseID is used to differentiate between the
// official SFlow standard, and other, vendor-specific
// types of flow data. (Similiar to SNMP's enterprise MIB
// OIDs) Only the office SFlow Enterprise ID is decoded
// here.
type SFlowEnterpriseID uint32

const (
	SFlowStandard SFlowEnterpriseID = 0
)

func (eid SFlowEnterpriseID) String() string {
	switch eid {
	case SFlowStandard:
		return "Standard SFlow"
	default:
		return ""
	}
}

func (eid SFlowEnterpriseID) GetType() SFlowEnterpriseID {
	return SFlowStandard
}

// SFlowSampleType specifies the type of sample. Only flow samples
// and counter samples are supported
type SFlowSampleType uint32

const (
	SFlowTypeFlowSample            SFlowSampleType = 1
	SFlowTypeCounterSample         SFlowSampleType = 2
	SFlowTypeExpandedFlowSample    SFlowSampleType = 3
	SFlowTypeExpandedCounterSample SFlowSampleType = 4
)

func (st SFlowSampleType) GetType() SFlowSampleType {
	switch st {
	case SFlowTypeFlowSample:
		return SFlowTypeFlowSample
	case SFlowTypeCounterSample:
		return SFlowTypeCounterSample
	case SFlowTypeExpandedFlowSample:
		return SFlowTypeExpandedFlowSample
	case SFlowTypeExpandedCounterSample:
		return SFlowTypeExpandedCounterSample
	default:
		panic("Invalid Sample Type")
	}
}

func (st SFlowSampleType) String() string {
	switch st {
	case SFlowTypeFlowSample:
		return "Flow Sample"
	case SFlowTypeCounterSample:
		return "Counter Sample"
	case SFlowTypeExpandedFlowSample:
		return "Expanded Flow Sample"
	case SFlowTypeExpandedCounterSample:
		return "Expanded Counter Sample"
	default:
		return ""
	}
}

func (s *SFlowDatagram) LayerType() gopacket.LayerType { return LayerTypeSFlow }

func (d *SFlowDatagram) Payload() []byte { return nil }

func (d *SFlowDatagram) CanDecode() gopacket.LayerClass { return LayerTypeSFlow }

func (d *SFlowDatagram) NextLayerType() gopacket.LayerType { return gopacket.LayerTypePayload }

// SFlowIPType determines what form the IP address being decoded will
// take. This is an XDR union type allowing for both IPv4 and IPv6
type SFlowIPType uint32

const (
	SFlowIPv4 SFlowIPType = 1
	SFlowIPv6 SFlowIPType = 2
)

func (s SFlowIPType) String() string {
	switch s {
	case SFlowIPv4:
		return "IPv4"
	case SFlowIPv6:
		return "IPv6"
	default:
		return ""
	}
}

func (s SFlowIPType) Length() int {
	switch s {
	case SFlowIPv4:
		return 4
	case SFlowIPv6:
		return 16
	default:
		return 0
	}
}

func (s *SFlowDatagram) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	var agentAddressType SFlowIPType

	data, s.DatagramVersion = data[4:], binary.BigEndian.Uint32(data[:4])
	data, agentAddressType = data[4:], SFlowIPType(binary.BigEndian.Uint32(data[:4]))
	data, s.AgentAddress = data[agentAddressType.Length():], data[:agentAddressType.Length()]
	data, s.SubAgentID = data[4:], binary.BigEndian.Uint32(data[:4])
	data, s.SequenceNumber = data[4:], binary.BigEndian.Uint32(data[:4])
	data, s.AgentUptime = data[4:], binary.BigEndian.Uint32(data[:4])
	data, s.SampleCount = data[4:], binary.BigEndian.Uint32(data[:4])

	if s.SampleCount < 1 {
		return fmt.Errorf("SFlow Datagram has invalid sample length: %d", s.SampleCount)
	}
	for i := uint32(0); i < s.SampleCount; i++ {
		sdf := SFlowDataFormat(binary.BigEndian.Uint32(data[:4]))
		_, sampleType := sdf.decode()
		switch sampleType {
		case SFlowTypeFlowSample:
			if flowSample, err := decodeFlowSample(&data, false); err == nil {
				s.FlowSamples = append(s.FlowSamples, flowSample)
			} else {
				return err
			}
		case SFlowTypeCounterSample:
			if counterSample, err := decodeCounterSample(&data, false); err == nil {
				s.CounterSamples = append(s.CounterSamples, counterSample)
			} else {
				return err
			}
		case SFlowTypeExpandedFlowSample:
			if flowSample, err := decodeFlowSample(&data, true); err == nil {
				s.FlowSamples = append(s.FlowSamples, flowSample)
			} else {
				return err
			}
		case SFlowTypeExpandedCounterSample:
			if counterSample, err := decodeCounterSample(&data, true); err == nil {
				s.CounterSamples = append(s.CounterSamples, counterSample)
			} else {
				return err
			}

		default:
			return fmt.Errorf("Unsupported SFlow sample type %d", sampleType)
		}
	}
	return nil
}

// SFlowFlowSample represents a sampled packet and contains
// one or more records describing the packet
type SFlowFlowSample struct {
	EnterpriseID          SFlowEnterpriseID
	Format                SFlowSampleType
	SampleLength          uint32
	SequenceNumber        uint32
	SourceIDClass         SFlowSourceFormat
	SourceIDIndex         SFlowSourceValue
	SamplingRate          uint32
	SamplePool            uint32
	Dropped               uint32
	InputInterfaceFormat  uint32
	InputInterface        uint32
	OutputInterfaceFormat uint32
	OutputInterface       uint32
	RecordCount           uint32
	Records               []SFlowRecord
}

// Flow samples have the following structure. Note
// the bit fields to encode the Enterprise ID and the
// Flow record format: type 1

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  sample length                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |          int sample sequence number           |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |id type |       src id index value             |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |               int sampling rate               |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                int sample pool                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                    int drops                  |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                 int input ifIndex             |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                int output ifIndex             |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |               int number of records           |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  /                   flow records                /
//  /                                               /
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

// Flow samples have the following structure.
// Flow record format: type 3

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  sample length                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |          int sample sequence number           |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |               int src id type                 |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |             int src id index value            |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |               int sampling rate               |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                int sample pool                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                    int drops                  |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |           int input interface format          |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |           int input interface value           |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |           int output interface format         |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |           int output interface value          |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |               int number of records           |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  /                   flow records                /
//  /                                               /
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

type SFlowFlowDataFormat uint32

func (fdf SFlowFlowDataFormat) decode() (SFlowEnterpriseID, SFlowFlowRecordType) {
	leftField := fdf >> 12
	rightField := uint32(0xFFF) & uint32(fdf)
	return SFlowEnterpriseID(leftField), SFlowFlowRecordType(rightField)
}

func (fs SFlowFlowSample) GetRecords() []SFlowRecord {
	return fs.Records
}

func (fs SFlowFlowSample) GetType() SFlowSampleType {
	return SFlowTypeFlowSample
}

func skipRecord(data *[]byte) {
	recordLength := int(binary.BigEndian.Uint32((*data)[4:]))
	*data = (*data)[(recordLength+((4-recordLength)%4))+8:]
}

func decodeFlowSample(data *[]byte, expanded bool) (SFlowFlowSample, error) {
	s := SFlowFlowSample{}
	var sdf SFlowDataFormat
	*data, sdf = (*data)[4:], SFlowDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	var sdc SFlowDataSource

	s.EnterpriseID, s.Format = sdf.decode()
	if len(*data) < 4 {
		return SFlowFlowSample{}, errors.New("ethernet counters too small")
	}
	*data, s.SampleLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	if len(*data) < 4 {
		return SFlowFlowSample{}, errors.New("ethernet counters too small")
	}
	*data, s.SequenceNumber = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	if expanded {
		if len(*data) < 4 {
			return SFlowFlowSample{}, errors.New("ethernet counters too small")
		}
		*data, s.SourceIDClass = (*data)[4:], SFlowSourceFormat(binary.BigEndian.Uint32((*data)[:4]))
		if len(*data) < 4 {
			return SFlowFlowSample{}, errors.New("ethernet counters too small")
		}
		*data, s.SourceIDIndex = (*data)[4:], SFlowSourceValue(binary.BigEndian.Uint32((*data)[:4]))
	} else {
		if len(*data) < 4 {
			return SFlowFlowSample{}, errors.New("ethernet counters too small")
		}
		*data, sdc = (*data)[4:], SFlowDataSource(binary.BigEndian.Uint32((*data)[:4]))
		s.SourceIDClass, s.SourceIDIndex = sdc.decode()
	}
	if len(*data) < 4 {
		return SFlowFlowSample{}, errors.New("ethernet counters too small")
	}
	*data, s.SamplingRate = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	if len(*data) < 4 {
		return SFlowFlowSample{}, errors.New("ethernet counters too small")
	}
	*data, s.SamplePool = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	if len(*data) < 4 {
		return SFlowFlowSample{}, errors.New("ethernet counters too small")
	}
	*data, s.Dropped = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])

	if expanded {
		if len(*data) < 4 {
			return SFlowFlowSample{}, errors.New("ethernet counters too small")
		}
		*data, s.InputInterfaceFormat = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
		if len(*data) < 4 {
			return SFlowFlowSample{}, errors.New("ethernet counters too small")
		}
		*data, s.InputInterface = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
		if len(*data) < 4 {
			return SFlowFlowSample{}, errors.New("ethernet counters too small")
		}
		*data, s.OutputInterfaceFormat = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
		if len(*data) < 4 {
			return SFlowFlowSample{}, errors.New("ethernet counters too small")
		}
		*data, s.OutputInterface = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	} else {
		if len(*data) < 4 {
			return SFlowFlowSample{}, errors.New("ethernet counters too small")
		}
		*data, s.InputInterface = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
		if len(*data) < 4 {
			return SFlowFlowSample{}, errors.New("ethernet counters too small")
		}
		*data, s.OutputInterface = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	}
	if len(*data) < 4 {
		return SFlowFlowSample{}, errors.New("ethernet counters too small")
	}
	*data, s.RecordCount = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])

	for i := uint32(0); i < s.RecordCount; i++ {
		rdf := SFlowFlowDataFormat(binary.BigEndian.Uint32((*data)[:4]))
		enterpriseID, flowRecordType := rdf.decode()

		// Try to decode when EnterpriseID is 0 signaling
		// default sflow structs are used according specification
		// Unexpected behavior detected for e.g. with pmacct
		if enterpriseID == 0 {
			switch flowRecordType {
			case SFlowTypeRawPacketFlow:
				if record, err := decodeRawPacketFlowRecord(data); err == nil {
					s.Records = append(s.Records, record)
				} else {
					return s, err
				}
			case SFlowTypeExtendedUserFlow:
				if record, err := decodeExtendedUserFlow(data); err == nil {
					s.Records = append(s.Records, record)
				} else {
					return s, err
				}
			case SFlowTypeExtendedUrlFlow:
				if record, err := decodeExtendedURLRecord(data); err == nil {
					s.Records = append(s.Records, record)
				} else {
					return s, err
				}
			case SFlowTypeExtendedSwitchFlow:
				if record, err := decodeExtendedSwitchFlowRecord(data); err == nil {
					s.Records = append(s.Records, record)
				} else {
					return s, err
				}
			case SFlowTypeExtendedRouterFlow:
				if record, err := decodeExtendedRouterFlowRecord(data); err == nil {
					s.Records = append(s.Records, record)
				} else {
					return s, err
				}
			case SFlowTypeExtendedGatewayFlow:
				if record, err := decodeExtendedGatewayFlowRecord(data); err == nil {
					s.Records = append(s.Records, record)
				} else {
					return s, err
				}
			case SFlowTypeEthernetFrameFlow:
				if record, err := decodeEthernetFrameFlowRecord(data); err == nil {
					s.Records = append(s.Records, record)
				} else {
					return s, err
				}
			case SFlowTypeIpv4Flow:
				if record, err := decodeSFlowIpv4Record(data); err == nil {
					s.Records = append(s.Records, record)
				} else {
					return s, err
				}
			case SFlowTypeIpv6Flow:
				if record, err := decodeSFlowIpv6Record(data); err == nil {
					s.Records = append(s.Records, record)
				} else {
					return s, err
				}
			case SFlowTypeExtendedMlpsFlow:
				// TODO
				skipRecord(data)
				return s, errors.New("skipping TypeExtendedMlpsFlow")
			case SFlowTypeExtendedNatFlow:
				// TODO
				skipRecord(data)
				return s, errors.New("skipping TypeExtendedNatFlow")
			case SFlowTypeExtendedMlpsTunnelFlow:
				// TODO
				skipRecord(data)
				return s, errors.New("skipping TypeExtendedMlpsTunnelFlow")
			case SFlowTypeExtendedMlpsVcFlow:
				// TODO
				skipRecord(data)
				return s, errors.New("skipping TypeExtendedMlpsVcFlow")
			case SFlowTypeExtendedMlpsFecFlow:
				// TODO
				skipRecord(data)
				return s, errors.New("skipping TypeExtendedMlpsFecFlow")
			case SFlowTypeExtendedMlpsLvpFecFlow:
				// TODO
				skipRecord(data)
				return s, errors.New("skipping TypeExtendedMlpsLvpFecFlow")
			case SFlowTypeExtendedVlanFlow:
				// TODO
				skipRecord(data)
				return s, errors.New("skipping TypeExtendedVlanFlow")
			case SFlowTypeExtendedIpv4TunnelEgressFlow:
				if record, err := decodeExtendedIpv4TunnelEgress(data); err == nil {
					s.Records = append(s.Records, record)
				} else {
					return s, err
				}
			case SFlowTypeExtendedIpv4TunnelIngressFlow:
				if record, err := decodeExtendedIpv4TunnelIngress(data); err == nil {
					s.Records = append(s.Records, record)
				} else {
					return s, err
				}
			case SFlowTypeExtendedIpv6TunnelEgressFlow:
				if record, err := decodeExtendedIpv6TunnelEgress(data); err == nil {
					s.Records = append(s.Records, record)
				} else {
					return s, err
				}
			case SFlowTypeExtendedIpv6TunnelIngressFlow:
				if record, err := decodeExtendedIpv6TunnelIngress(data); err == nil {
					s.Records = append(s.Records, record)
				} else {
					return s, err
				}
			case SFlowTypeExtendedDecapsulateEgressFlow:
				if record, err := decodeExtendedDecapsulateEgress(data); err == nil {
					s.Records = append(s.Records, record)
				} else {
					return s, err
				}
			case SFlowTypeExtendedDecapsulateIngressFlow:
				if record, err := decodeExtendedDecapsulateIngress(data); err == nil {
					s.Records = append(s.Records, record)
				} else {
					return s, err
				}
			case SFlowTypeExtendedVniEgressFlow:
				if record, err := decodeExtendedVniEgress(data); err == nil {
					s.Records = append(s.Records, record)
				} else {
					return s, err
				}
			case SFlowTypeExtendedVniIngressFlow:
				if record, err := decodeExtendedVniIngress(data); err == nil {
					s.Records = append(s.Records, record)
				} else {
					return s, err
				}
			default:
				return s, fmt.Errorf("Unsupported flow record type: %d", flowRecordType)
			}
		} else {
			skipRecord(data)
		}
	}
	return s, nil
}

// Counter samples report information about various counter
// objects. Typically these are items like IfInOctets, or
// CPU / Memory stats, etc. SFlow will report these at regular
// intervals as configured on the agent. If one were sufficiently
// industrious, this could be used to replace the typical
// SNMP polling used for such things.
type SFlowCounterSample struct {
	EnterpriseID   SFlowEnterpriseID
	Format         SFlowSampleType
	SampleLength   uint32
	SequenceNumber uint32
	SourceIDClass  SFlowSourceFormat
	SourceIDIndex  SFlowSourceValue
	RecordCount    uint32
	Records        []SFlowRecord
}

// Counter samples have the following structure:

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |          int sample sequence number           |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |id type |       src id index value             |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |               int number of records           |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  /                counter records                /
//  /                                               /
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

type SFlowCounterDataFormat uint32

func (cdf SFlowCounterDataFormat) decode() (SFlowEnterpriseID, SFlowCounterRecordType) {
	leftField := cdf >> 12
	rightField := uint32(0xFFF) & uint32(cdf)
	return SFlowEnterpriseID(leftField), SFlowCounterRecordType(rightField)
}

// GetRecords will return a slice of interface types
// representing records. A type switch can be used to
// get at the underlying SFlowCounterRecordType.
func (cs SFlowCounterSample) GetRecords() []SFlowRecord {
	return cs.Records
}

// GetType will report the type of sample. Only the
// compact form of counter samples is supported
func (cs SFlowCounterSample) GetType() SFlowSampleType {
	return SFlowTypeCounterSample
}

type SFlowCounterRecordType uint32

const (
	SFlowTypeGenericInterfaceCounters   SFlowCounterRecordType = 1
	SFlowTypeEthernetInterfaceCounters  SFlowCounterRecordType = 2
	SFlowTypeTokenRingInterfaceCounters SFlowCounterRecordType = 3
	SFlowType100BaseVGInterfaceCounters SFlowCounterRecordType = 4
	SFlowTypeVLANCounters               SFlowCounterRecordType = 5
	SFlowTypeLACPCounters               SFlowCounterRecordType = 7
	SFlowTypeProcessorCounters          SFlowCounterRecordType = 1001
	SFlowTypeOpenflowPortCounters       SFlowCounterRecordType = 1004
	SFlowTypePORTNAMECounters           SFlowCounterRecordType = 1005
	SFLowTypeAPPRESOURCESCounters       SFlowCounterRecordType = 2203
	SFlowTypeOVSDPCounters              SFlowCounterRecordType = 2207
)

func (cr SFlowCounterRecordType) String() string {
	switch cr {
	case SFlowTypeGenericInterfaceCounters:
		return "Generic Interface Counters"
	case SFlowTypeEthernetInterfaceCounters:
		return "Ethernet Interface Counters"
	case SFlowTypeTokenRingInterfaceCounters:
		return "Token Ring Interface Counters"
	case SFlowType100BaseVGInterfaceCounters:
		return "100BaseVG Interface Counters"
	case SFlowTypeVLANCounters:
		return "VLAN Counters"
	case SFlowTypeLACPCounters:
		return "LACP Counters"
	case SFlowTypeProcessorCounters:
		return "Processor Counters"
	case SFlowTypeOpenflowPortCounters:
		return "Openflow Port Counters"
	case SFlowTypePORTNAMECounters:
		return "PORT NAME Counters"
	case SFLowTypeAPPRESOURCESCounters:
		return "App Resources Counters"
	case SFlowTypeOVSDPCounters:
		return "OVSDP Counters"
	default:
		return ""

	}
}

func decodeCounterSample(data *[]byte, expanded bool) (SFlowCounterSample, error) {
	s := SFlowCounterSample{}
	var sdc SFlowDataSource
	var sdce SFlowDataSourceExpanded
	var sdf SFlowDataFormat

	*data, sdf = (*data)[4:], SFlowDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	s.EnterpriseID, s.Format = sdf.decode()
	*data, s.SampleLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, s.SequenceNumber = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	if expanded {
		*data, sdce = (*data)[8:], SFlowDataSourceExpanded{SFlowSourceFormat(binary.BigEndian.Uint32((*data)[:4])), SFlowSourceValue(binary.BigEndian.Uint32((*data)[4:8]))}
		s.SourceIDClass, s.SourceIDIndex = sdce.decode()
	} else {
		*data, sdc = (*data)[4:], SFlowDataSource(binary.BigEndian.Uint32((*data)[:4]))
		s.SourceIDClass, s.SourceIDIndex = sdc.decode()
	}
	*data, s.RecordCount = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])

	for i := uint32(0); i < s.RecordCount; i++ {
		cdf := SFlowCounterDataFormat(binary.BigEndian.Uint32((*data)[:4]))
		_, counterRecordType := cdf.decode()
		switch counterRecordType {
		case SFlowTypeGenericInterfaceCounters:
			if record, err := decodeGenericInterfaceCounters(data); err == nil {
				s.Records = append(s.Records, record)
			} else {
				return s, err
			}
		case SFlowTypeEthernetInterfaceCounters:
			if record, err := decodeEthernetCounters(data); err == nil {
				s.Records = append(s.Records, record)
			} else {
				return s, err
			}
		case SFlowTypeTokenRingInterfaceCounters:
			skipRecord(data)
			return s, errors.New("skipping TypeTokenRingInterfaceCounters")
		case SFlowType100BaseVGInterfaceCounters:
			skipRecord(data)
			return s, errors.New("skipping Type100BaseVGInterfaceCounters")
		case SFlowTypeVLANCounters:
			if record, err := decodeVLANCounters(data); err == nil {
				s.Records = append(s.Records, record)
			} else {
				return s, err
			}
		case SFlowTypeLACPCounters:
			if record, err := decodeLACPCounters(data); err == nil {
				s.Records = append(s.Records, record)
			} else {
				return s, err
			}
		case SFlowTypeProcessorCounters:
			if record, err := decodeProcessorCounters(data); err == nil {
				s.Records = append(s.Records, record)
			} else {
				return s, err
			}
		case SFlowTypeOpenflowPortCounters:
			if record, err := decodeOpenflowportCounters(data); err == nil {
				s.Records = append(s.Records, record)
			} else {
				return s, err
			}
		case SFlowTypePORTNAMECounters:
			if record, err := decodePortnameCounters(data); err == nil {
				s.Records = append(s.Records, record)
			} else {
				return s, err
			}
		case SFLowTypeAPPRESOURCESCounters:
			if record, err := decodeAppresourcesCounters(data); err == nil {
				s.Records = append(s.Records, record)
			} else {
				return s, err
			}
		case SFlowTypeOVSDPCounters:
			if record, err := decodeOVSDPCounters(data); err == nil {
				s.Records = append(s.Records, record)
			} else {
				return s, err
			}
		default:
			return s, fmt.Errorf("Invalid counter record type: %d", counterRecordType)
		}
	}
	return s, nil
}

// SFlowBaseFlowRecord holds the fields common to all records
// of type SFlowFlowRecordType
type SFlowBaseFlowRecord struct {
	EnterpriseID   SFlowEnterpriseID
	Format         SFlowFlowRecordType
	FlowDataLength uint32
}

func (bfr SFlowBaseFlowRecord) GetType() SFlowFlowRecordType {
	return bfr.Format
}

// SFlowFlowRecordType denotes what kind of Flow Record is
// represented. See RFC 3176
type SFlowFlowRecordType uint32

const (
	SFlowTypeRawPacketFlow                  SFlowFlowRecordType = 1
	SFlowTypeEthernetFrameFlow              SFlowFlowRecordType = 2
	SFlowTypeIpv4Flow                       SFlowFlowRecordType = 3
	SFlowTypeIpv6Flow                       SFlowFlowRecordType = 4
	SFlowTypeExtendedSwitchFlow             SFlowFlowRecordType = 1001
	SFlowTypeExtendedRouterFlow             SFlowFlowRecordType = 1002
	SFlowTypeExtendedGatewayFlow            SFlowFlowRecordType = 1003
	SFlowTypeExtendedUserFlow               SFlowFlowRecordType = 1004
	SFlowTypeExtendedUrlFlow                SFlowFlowRecordType = 1005
	SFlowTypeExtendedMlpsFlow               SFlowFlowRecordType = 1006
	SFlowTypeExtendedNatFlow                SFlowFlowRecordType = 1007
	SFlowTypeExtendedMlpsTunnelFlow         SFlowFlowRecordType = 1008
	SFlowTypeExtendedMlpsVcFlow             SFlowFlowRecordType = 1009
	SFlowTypeExtendedMlpsFecFlow            SFlowFlowRecordType = 1010
	SFlowTypeExtendedMlpsLvpFecFlow         SFlowFlowRecordType = 1011
	SFlowTypeExtendedVlanFlow               SFlowFlowRecordType = 1012
	SFlowTypeExtendedIpv4TunnelEgressFlow   SFlowFlowRecordType = 1023
	SFlowTypeExtendedIpv4TunnelIngressFlow  SFlowFlowRecordType = 1024
	SFlowTypeExtendedIpv6TunnelEgressFlow   SFlowFlowRecordType = 1025
	SFlowTypeExtendedIpv6TunnelIngressFlow  SFlowFlowRecordType = 1026
	SFlowTypeExtendedDecapsulateEgressFlow  SFlowFlowRecordType = 1027
	SFlowTypeExtendedDecapsulateIngressFlow SFlowFlowRecordType = 1028
	SFlowTypeExtendedVniEgressFlow          SFlowFlowRecordType = 1029
	SFlowTypeExtendedVniIngressFlow         SFlowFlowRecordType = 1030
)

func (rt SFlowFlowRecordType) String() string {
	switch rt {
	case SFlowTypeRawPacketFlow:
		return "Raw Packet Flow Record"
	case SFlowTypeEthernetFrameFlow:
		return "Ethernet Frame Flow Record"
	case SFlowTypeIpv4Flow:
		return "IPv4 Flow Record"
	case SFlowTypeIpv6Flow:
		return "IPv6 Flow Record"
	case SFlowTypeExtendedSwitchFlow:
		return "Extended Switch Flow Record"
	case SFlowTypeExtendedRouterFlow:
		return "Extended Router Flow Record"
	case SFlowTypeExtendedGatewayFlow:
		return "Extended Gateway Flow Record"
	case SFlowTypeExtendedUserFlow:
		return "Extended User Flow Record"
	case SFlowTypeExtendedUrlFlow:
		return "Extended URL Flow Record"
	case SFlowTypeExtendedMlpsFlow:
		return "Extended MPLS Flow Record"
	case SFlowTypeExtendedNatFlow:
		return "Extended NAT Flow Record"
	case SFlowTypeExtendedMlpsTunnelFlow:
		return "Extended MPLS Tunnel Flow Record"
	case SFlowTypeExtendedMlpsVcFlow:
		return "Extended MPLS VC Flow Record"
	case SFlowTypeExtendedMlpsFecFlow:
		return "Extended MPLS FEC Flow Record"
	case SFlowTypeExtendedMlpsLvpFecFlow:
		return "Extended MPLS LVP FEC Flow Record"
	case SFlowTypeExtendedVlanFlow:
		return "Extended VLAN Flow Record"
	case SFlowTypeExtendedIpv4TunnelEgressFlow:
		return "Extended IPv4 Tunnel Egress Record"
	case SFlowTypeExtendedIpv4TunnelIngressFlow:
		return "Extended IPv4 Tunnel Ingress Record"
	case SFlowTypeExtendedIpv6TunnelEgressFlow:
		return "Extended IPv6 Tunnel Egress Record"
	case SFlowTypeExtendedIpv6TunnelIngressFlow:
		return "Extended IPv6 Tunnel Ingress Record"
	case SFlowTypeExtendedDecapsulateEgressFlow:
		return "Extended Decapsulate Egress Record"
	case SFlowTypeExtendedDecapsulateIngressFlow:
		return "Extended Decapsulate Ingress Record"
	case SFlowTypeExtendedVniEgressFlow:
		return "Extended VNI Ingress Record"
	case SFlowTypeExtendedVniIngressFlow:
		return "Extended VNI Ingress Record"
	default:
		return ""
	}
}

// SFlowRawPacketFlowRecords hold information about a sampled
// packet grabbed as it transited the agent. This is
// perhaps the most useful and interesting record type,
// as it holds the headers of the sampled packet and
// can be used to build up a complete picture of the
// traffic patterns on a network.
//
// The raw packet header is sent back into gopacket for
// decoding, and the resulting gopackt.Packet is stored
// in the Header member
type SFlowRawPacketFlowRecord struct {
	SFlowBaseFlowRecord
	HeaderProtocol SFlowRawHeaderProtocol
	FrameLength    uint32
	PayloadRemoved uint32
	HeaderLength   uint32
	Header         gopacket.Packet
}

// Raw packet record types have the following structure:

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  record length                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                 Header Protocol               |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                 Frame Length                  |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                 Payload Removed               |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                 Header Length                 |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  \                     Header                    \
//  \                                               \
//  \                                               \
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

type SFlowRawHeaderProtocol uint32

const (
	SFlowProtoEthernet   SFlowRawHeaderProtocol = 1
	SFlowProtoISO88024   SFlowRawHeaderProtocol = 2
	SFlowProtoISO88025   SFlowRawHeaderProtocol = 3
	SFlowProtoFDDI       SFlowRawHeaderProtocol = 4
	SFlowProtoFrameRelay SFlowRawHeaderProtocol = 5
	SFlowProtoX25        SFlowRawHeaderProtocol = 6
	SFlowProtoPPP        SFlowRawHeaderProtocol = 7
	SFlowProtoSMDS       SFlowRawHeaderProtocol = 8
	SFlowProtoAAL5       SFlowRawHeaderProtocol = 9
	SFlowProtoAAL5_IP    SFlowRawHeaderProtocol = 10 /* e.g. Cisco AAL5 mux */
	SFlowProtoIPv4       SFlowRawHeaderProtocol = 11
	SFlowProtoIPv6       SFlowRawHeaderProtocol = 12
	SFlowProtoMPLS       SFlowRawHeaderProtocol = 13
	SFlowProtoPOS        SFlowRawHeaderProtocol = 14 /* RFC 1662, 2615 */
)

func (sfhp SFlowRawHeaderProtocol) String() string {
	switch sfhp {
	case SFlowProtoEthernet:
		return "ETHERNET-ISO88023"
	case SFlowProtoISO88024:
		return "ISO88024-TOKENBUS"
	case SFlowProtoISO88025:
		return "ISO88025-TOKENRING"
	case SFlowProtoFDDI:
		return "FDDI"
	case SFlowProtoFrameRelay:
		return "FRAME-RELAY"
	case SFlowProtoX25:
		return "X25"
	case SFlowProtoPPP:
		return "PPP"
	case SFlowProtoSMDS:
		return "SMDS"
	case SFlowProtoAAL5:
		return "AAL5"
	case SFlowProtoAAL5_IP:
		return "AAL5-IP"
	case SFlowProtoIPv4:
		return "IPv4"
	case SFlowProtoIPv6:
		return "IPv6"
	case SFlowProtoMPLS:
		return "MPLS"
	case SFlowProtoPOS:
		return "POS"
	}
	return "UNKNOWN"
}

func decodeRawPacketFlowRecord(data *[]byte) (SFlowRawPacketFlowRecord, error) {
	rec := SFlowRawPacketFlowRecord{}
	header := []byte{}
	var fdf SFlowFlowDataFormat

	*data, fdf = (*data)[4:], SFlowFlowDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	rec.EnterpriseID, rec.Format = fdf.decode()
	*data, rec.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, rec.HeaderProtocol = (*data)[4:], SFlowRawHeaderProtocol(binary.BigEndian.Uint32((*data)[:4]))
	*data, rec.FrameLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, rec.PayloadRemoved = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, rec.HeaderLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	headerLenWithPadding := int(rec.HeaderLength + ((4 - rec.HeaderLength) % 4))
	*data, header = (*data)[headerLenWithPadding:], (*data)[:headerLenWithPadding]
	rec.Header = gopacket.NewPacket(header, LayerTypeEthernet, gopacket.Default)
	return rec, nil
}

// SFlowExtendedSwitchFlowRecord give additional information
// about the sampled packet if it's available. It's mainly
// useful for getting at the incoming and outgoing VLANs
// An agent may or may not provide this information.
type SFlowExtendedSwitchFlowRecord struct {
	SFlowBaseFlowRecord
	IncomingVLAN         uint32
	IncomingVLANPriority uint32
	OutgoingVLAN         uint32
	OutgoingVLANPriority uint32
}

// Extended switch records have the following structure:

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  record length                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                   Incoming VLAN               |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                Incoming VLAN Priority         |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                   Outgoing VLAN               |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                Outgoing VLAN Priority         |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

func decodeExtendedSwitchFlowRecord(data *[]byte) (SFlowExtendedSwitchFlowRecord, error) {
	es := SFlowExtendedSwitchFlowRecord{}
	var fdf SFlowFlowDataFormat

	*data, fdf = (*data)[4:], SFlowFlowDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	es.EnterpriseID, es.Format = fdf.decode()
	*data, es.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, es.IncomingVLAN = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, es.IncomingVLANPriority = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, es.OutgoingVLAN = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, es.OutgoingVLANPriority = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	return es, nil
}

// SFlowExtendedRouterFlowRecord gives additional information
// about the layer 3 routing information used to forward
// the packet
type SFlowExtendedRouterFlowRecord struct {
	SFlowBaseFlowRecord
	NextHop                net.IP
	NextHopSourceMask      uint32
	NextHopDestinationMask uint32
}

// Extended router records have the following structure:

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  record length                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |   IP version of next hop router (1=v4|2=v6)   |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  /     Next Hop address (v4=4byte|v6=16byte)     /
//  /                                               /
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |              Next Hop Source Mask             |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |              Next Hop Destination Mask        |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

func decodeExtendedRouterFlowRecord(data *[]byte) (SFlowExtendedRouterFlowRecord, error) {
	er := SFlowExtendedRouterFlowRecord{}
	var fdf SFlowFlowDataFormat
	var extendedRouterAddressType SFlowIPType

	*data, fdf = (*data)[4:], SFlowFlowDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	er.EnterpriseID, er.Format = fdf.decode()
	*data, er.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, extendedRouterAddressType = (*data)[4:], SFlowIPType(binary.BigEndian.Uint32((*data)[:4]))
	*data, er.NextHop = (*data)[extendedRouterAddressType.Length():], (*data)[:extendedRouterAddressType.Length()]
	*data, er.NextHopSourceMask = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, er.NextHopDestinationMask = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	return er, nil
}

// SFlowExtendedGatewayFlowRecord describes information treasured by
// nework engineers everywhere: AS path information listing which
// BGP peer sent the packet, and various other BGP related info.
// This information is vital because it gives a picture of how much
// traffic is being sent from / received by various BGP peers.

// Extended gateway records have the following structure:

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  record length                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |   IP version of next hop router (1=v4|2=v6)   |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  /     Next Hop address (v4=4byte|v6=16byte)     /
//  /                                               /
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                       AS                      |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  Source AS                    |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                    Peer AS                    |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  AS Path Count                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  /                AS Path / Sequence             /
//  /                                               /
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  /                   Communities                 /
//  /                                               /
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                    Local Pref                 |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

// AS Path / Sequence:

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |     AS Source Type (Path=1 / Sequence=2)      |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |              Path / Sequence length           |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  /              Path / Sequence Members          /
//  /                                               /
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

// Communities:

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                communitiy length              |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  /              communitiy Members               /
//  /                                               /
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

type SFlowExtendedGatewayFlowRecord struct {
	SFlowBaseFlowRecord
	NextHop     net.IP
	AS          uint32
	SourceAS    uint32
	PeerAS      uint32
	ASPathCount uint32
	ASPath      []SFlowASDestination
	Communities []uint32
	LocalPref   uint32
}

type SFlowASPathType uint32

const (
	SFlowASSet      SFlowASPathType = 1
	SFlowASSequence SFlowASPathType = 2
)

func (apt SFlowASPathType) String() string {
	switch apt {
	case SFlowASSet:
		return "AS Set"
	case SFlowASSequence:
		return "AS Sequence"
	default:
		return ""
	}
}

type SFlowASDestination struct {
	Type    SFlowASPathType
	Count   uint32
	Members []uint32
}

func (asd SFlowASDestination) String() string {
	switch asd.Type {
	case SFlowASSet:
		return fmt.Sprint("AS Set:", asd.Members)
	case SFlowASSequence:
		return fmt.Sprint("AS Sequence:", asd.Members)
	default:
		return ""
	}
}

func (ad *SFlowASDestination) decodePath(data *[]byte) {
	*data, ad.Type = (*data)[4:], SFlowASPathType(binary.BigEndian.Uint32((*data)[:4]))
	*data, ad.Count = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	ad.Members = make([]uint32, ad.Count)
	for i := uint32(0); i < ad.Count; i++ {
		var member uint32
		*data, member = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
		ad.Members[i] = member
	}
}

func decodeExtendedGatewayFlowRecord(data *[]byte) (SFlowExtendedGatewayFlowRecord, error) {
	eg := SFlowExtendedGatewayFlowRecord{}
	var fdf SFlowFlowDataFormat
	var extendedGatewayAddressType SFlowIPType
	var communitiesLength uint32
	var community uint32

	*data, fdf = (*data)[4:], SFlowFlowDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	eg.EnterpriseID, eg.Format = fdf.decode()
	*data, eg.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, extendedGatewayAddressType = (*data)[4:], SFlowIPType(binary.BigEndian.Uint32((*data)[:4]))
	*data, eg.NextHop = (*data)[extendedGatewayAddressType.Length():], (*data)[:extendedGatewayAddressType.Length()]
	*data, eg.AS = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, eg.SourceAS = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, eg.PeerAS = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, eg.ASPathCount = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	for i := uint32(0); i < eg.ASPathCount; i++ {
		asPath := SFlowASDestination{}
		asPath.decodePath(data)
		eg.ASPath = append(eg.ASPath, asPath)
	}
	*data, communitiesLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	eg.Communities = make([]uint32, communitiesLength)
	for j := uint32(0); j < communitiesLength; j++ {
		*data, community = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
		eg.Communities[j] = community
	}
	*data, eg.LocalPref = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	return eg, nil
}

// **************************************************
//  Extended URL Flow Record
// **************************************************

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  record length                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                   direction                   |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                      URL                      |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                      Host                     |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

type SFlowURLDirection uint32

const (
	SFlowURLsrc SFlowURLDirection = 1
	SFlowURLdst SFlowURLDirection = 2
)

func (urld SFlowURLDirection) String() string {
	switch urld {
	case SFlowURLsrc:
		return "Source address is the server"
	case SFlowURLdst:
		return "Destination address is the server"
	default:
		return ""
	}
}

type SFlowExtendedURLRecord struct {
	SFlowBaseFlowRecord
	Direction SFlowURLDirection
	URL       string
	Host      string
}

func decodeExtendedURLRecord(data *[]byte) (SFlowExtendedURLRecord, error) {
	eur := SFlowExtendedURLRecord{}
	var fdf SFlowFlowDataFormat
	var urlLen uint32
	var urlLenWithPad int
	var hostLen uint32
	var hostLenWithPad int
	var urlBytes []byte
	var hostBytes []byte

	*data, fdf = (*data)[4:], SFlowFlowDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	eur.EnterpriseID, eur.Format = fdf.decode()
	*data, eur.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, eur.Direction = (*data)[4:], SFlowURLDirection(binary.BigEndian.Uint32((*data)[:4]))
	*data, urlLen = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	urlLenWithPad = int(urlLen + ((4 - urlLen) % 4))
	*data, urlBytes = (*data)[urlLenWithPad:], (*data)[:urlLenWithPad]
	eur.URL = string(urlBytes[:urlLen])
	*data, hostLen = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	hostLenWithPad = int(hostLen + ((4 - hostLen) % 4))
	*data, hostBytes = (*data)[hostLenWithPad:], (*data)[:hostLenWithPad]
	eur.Host = string(hostBytes[:hostLen])
	return eur, nil
}

// **************************************************
//  Extended User Flow Record
// **************************************************

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  record length                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                Source Character Set           |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                 Source User Id                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |              Destination Character Set        |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |               Destination User ID             |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

type SFlowExtendedUserFlow struct {
	SFlowBaseFlowRecord
	SourceCharSet      SFlowCharSet
	SourceUserID       string
	DestinationCharSet SFlowCharSet
	DestinationUserID  string
}

type SFlowCharSet uint32

const (
	SFlowCSunknown                 SFlowCharSet = 2
	SFlowCSASCII                   SFlowCharSet = 3
	SFlowCSISOLatin1               SFlowCharSet = 4
	SFlowCSISOLatin2               SFlowCharSet = 5
	SFlowCSISOLatin3               SFlowCharSet = 6
	SFlowCSISOLatin4               SFlowCharSet = 7
	SFlowCSISOLatinCyrillic        SFlowCharSet = 8
	SFlowCSISOLatinArabic          SFlowCharSet = 9
	SFlowCSISOLatinGreek           SFlowCharSet = 10
	SFlowCSISOLatinHebrew          SFlowCharSet = 11
	SFlowCSISOLatin5               SFlowCharSet = 12
	SFlowCSISOLatin6               SFlowCharSet = 13
	SFlowCSISOTextComm             SFlowCharSet = 14
	SFlowCSHalfWidthKatakana       SFlowCharSet = 15
	SFlowCSJISEncoding             SFlowCharSet = 16
	SFlowCSShiftJIS                SFlowCharSet = 17
	SFlowCSEUCPkdFmtJapanese       SFlowCharSet = 18
	SFlowCSEUCFixWidJapanese       SFlowCharSet = 19
	SFlowCSISO4UnitedKingdom       SFlowCharSet = 20
	SFlowCSISO11SwedishForNames    SFlowCharSet = 21
	SFlowCSISO15Italian            SFlowCharSet = 22
	SFlowCSISO17Spanish            SFlowCharSet = 23
	SFlowCSISO21German             SFlowCharSet = 24
	SFlowCSISO60DanishNorwegian    SFlowCharSet = 25
	SFlowCSISO69French             SFlowCharSet = 26
	SFlowCSISO10646UTF1            SFlowCharSet = 27
	SFlowCSISO646basic1983         SFlowCharSet = 28
	SFlowCSINVARIANT               SFlowCharSet = 29
	SFlowCSISO2IntlRefVersion      SFlowCharSet = 30
	SFlowCSNATSSEFI                SFlowCharSet = 31
	SFlowCSNATSSEFIADD             SFlowCharSet = 32
	SFlowCSNATSDANO                SFlowCharSet = 33
	SFlowCSNATSDANOADD             SFlowCharSet = 34
	SFlowCSISO10Swedish            SFlowCharSet = 35
	SFlowCSKSC56011987             SFlowCharSet = 36
	SFlowCSISO2022KR               SFlowCharSet = 37
	SFlowCSEUCKR                   SFlowCharSet = 38
	SFlowCSISO2022JP               SFlowCharSet = 39
	SFlowCSISO2022JP2              SFlowCharSet = 40
	SFlowCSISO13JISC6220jp         SFlowCharSet = 41
	SFlowCSISO14JISC6220ro         SFlowCharSet = 42
	SFlowCSISO16Portuguese         SFlowCharSet = 43
	SFlowCSISO18Greek7Old          SFlowCharSet = 44
	SFlowCSISO19LatinGreek         SFlowCharSet = 45
	SFlowCSISO25French             SFlowCharSet = 46
	SFlowCSISO27LatinGreek1        SFlowCharSet = 47
	SFlowCSISO5427Cyrillic         SFlowCharSet = 48
	SFlowCSISO42JISC62261978       SFlowCharSet = 49
	SFlowCSISO47BSViewdata         SFlowCharSet = 50
	SFlowCSISO49INIS               SFlowCharSet = 51
	SFlowCSISO50INIS8              SFlowCharSet = 52
	SFlowCSISO51INISCyrillic       SFlowCharSet = 53
	SFlowCSISO54271981             SFlowCharSet = 54
	SFlowCSISO5428Greek            SFlowCharSet = 55
	SFlowCSISO57GB1988             SFlowCharSet = 56
	SFlowCSISO58GB231280           SFlowCharSet = 57
	SFlowCSISO61Norwegian2         SFlowCharSet = 58
	SFlowCSISO70VideotexSupp1      SFlowCharSet = 59
	SFlowCSISO84Portuguese2        SFlowCharSet = 60
	SFlowCSISO85Spanish2           SFlowCharSet = 61
	SFlowCSISO86Hungarian          SFlowCharSet = 62
	SFlowCSISO87JISX0208           SFlowCharSet = 63
	SFlowCSISO88Greek7             SFlowCharSet = 64
	SFlowCSISO89ASMO449            SFlowCharSet = 65
	SFlowCSISO90                   SFlowCharSet = 66
	SFlowCSISO91JISC62291984a      SFlowCharSet = 67
	SFlowCSISO92JISC62991984b      SFlowCharSet = 68
	SFlowCSISO93JIS62291984badd    SFlowCharSet = 69
	SFlowCSISO94JIS62291984hand    SFlowCharSet = 70
	SFlowCSISO95JIS62291984handadd SFlowCharSet = 71
	SFlowCSISO96JISC62291984kana   SFlowCharSet = 72
	SFlowCSISO2033                 SFlowCharSet = 73
	SFlowCSISO99NAPLPS             SFlowCharSet = 74
	SFlowCSISO102T617bit           SFlowCharSet = 75
	SFlowCSISO103T618bit           SFlowCharSet = 76
	SFlowCSISO111ECMACyrillic      SFlowCharSet = 77
	SFlowCSa71                     SFlowCharSet = 78
	SFlowCSa72                     SFlowCharSet = 79
	SFlowCSISO123CSAZ24341985gr    SFlowCharSet = 80
	SFlowCSISO88596E               SFlowCharSet = 81
	SFlowCSISO88596I               SFlowCharSet = 82
	SFlowCSISO128T101G2            SFlowCharSet = 83
	SFlowCSISO88598E               SFlowCharSet = 84
	SFlowCSISO88598I               SFlowCharSet = 85
	SFlowCSISO139CSN369103         SFlowCharSet = 86
	SFlowCSISO141JUSIB1002         SFlowCharSet = 87
	SFlowCSISO143IECP271           SFlowCharSet = 88
	SFlowCSISO146Serbian           SFlowCharSet = 89
	SFlowCSISO147Macedonian        SFlowCharSet = 90
	SFlowCSISO150                  SFlowCharSet = 91
	SFlowCSISO151Cuba              SFlowCharSet = 92
	SFlowCSISO6937Add              SFlowCharSet = 93
	SFlowCSISO153GOST1976874       SFlowCharSet = 94
	SFlowCSISO8859Supp             SFlowCharSet = 95
	SFlowCSISO10367Box             SFlowCharSet = 96
	SFlowCSISO158Lap               SFlowCharSet = 97
	SFlowCSISO159JISX02121990      SFlowCharSet = 98
	SFlowCSISO646Danish            SFlowCharSet = 99
	SFlowCSUSDK                    SFlowCharSet = 100
	SFlowCSDKUS                    SFlowCharSet = 101
	SFlowCSKSC5636                 SFlowCharSet = 102
	SFlowCSUnicode11UTF7           SFlowCharSet = 103
	SFlowCSISO2022CN               SFlowCharSet = 104
	SFlowCSISO2022CNEXT            SFlowCharSet = 105
	SFlowCSUTF8                    SFlowCharSet = 106
	SFlowCSISO885913               SFlowCharSet = 109
	SFlowCSISO885914               SFlowCharSet = 110
	SFlowCSISO885915               SFlowCharSet = 111
	SFlowCSISO885916               SFlowCharSet = 112
	SFlowCSGBK                     SFlowCharSet = 113
	SFlowCSGB18030                 SFlowCharSet = 114
	SFlowCSOSDEBCDICDF0415         SFlowCharSet = 115
	SFlowCSOSDEBCDICDF03IRV        SFlowCharSet = 116
	SFlowCSOSDEBCDICDF041          SFlowCharSet = 117
	SFlowCSISO115481               SFlowCharSet = 118
	SFlowCSKZ1048                  SFlowCharSet = 119
	SFlowCSUnicode                 SFlowCharSet = 1000
	SFlowCSUCS4                    SFlowCharSet = 1001
	SFlowCSUnicodeASCII            SFlowCharSet = 1002
	SFlowCSUnicodeLatin1           SFlowCharSet = 1003
	SFlowCSUnicodeJapanese         SFlowCharSet = 1004
	SFlowCSUnicodeIBM1261          SFlowCharSet = 1005
	SFlowCSUnicodeIBM1268          SFlowCharSet = 1006
	SFlowCSUnicodeIBM1276          SFlowCharSet = 1007
	SFlowCSUnicodeIBM1264          SFlowCharSet = 1008
	SFlowCSUnicodeIBM1265          SFlowCharSet = 1009
	SFlowCSUnicode11               SFlowCharSet = 1010
	SFlowCSSCSU                    SFlowCharSet = 1011
	SFlowCSUTF7                    SFlowCharSet = 1012
	SFlowCSUTF16BE                 SFlowCharSet = 1013
	SFlowCSUTF16LE                 SFlowCharSet = 1014
	SFlowCSUTF16                   SFlowCharSet = 1015
	SFlowCSCESU8                   SFlowCharSet = 1016
	SFlowCSUTF32                   SFlowCharSet = 1017
	SFlowCSUTF32BE                 SFlowCharSet = 1018
	SFlowCSUTF32LE                 SFlowCharSet = 1019
	SFlowCSBOCU1                   SFlowCharSet = 1020
	SFlowCSWindows30Latin1         SFlowCharSet = 2000
	SFlowCSWindows31Latin1         SFlowCharSet = 2001
	SFlowCSWindows31Latin2         SFlowCharSet = 2002
	SFlowCSWindows31Latin5         SFlowCharSet = 2003
	SFlowCSHPRoman8                SFlowCharSet = 2004
	SFlowCSAdobeStandardEncoding   SFlowCharSet = 2005
	SFlowCSVenturaUS               SFlowCharSet = 2006
	SFlowCSVenturaInternational    SFlowCharSet = 2007
	SFlowCSDECMCS                  SFlowCharSet = 2008
	SFlowCSPC850Multilingual       SFlowCharSet = 2009
	SFlowCSPCp852                  SFlowCharSet = 2010
	SFlowCSPC8CodePage437          SFlowCharSet = 2011
	SFlowCSPC8DanishNorwegian      SFlowCharSet = 2012
	SFlowCSPC862LatinHebrew        SFlowCharSet = 2013
	SFlowCSPC8Turkish              SFlowCharSet = 2014
	SFlowCSIBMSymbols              SFlowCharSet = 2015
	SFlowCSIBMThai                 SFlowCharSet = 2016
	SFlowCSHPLegal                 SFlowCharSet = 2017
	SFlowCSHPPiFont                SFlowCharSet = 2018
	SFlowCSHPMath8                 SFlowCharSet = 2019
	SFlowCSHPPSMath                SFlowCharSet = 2020
	SFlowCSHPDesktop               SFlowCharSet = 2021
	SFlowCSVenturaMath             SFlowCharSet = 2022
	SFlowCSMicrosoftPublishing     SFlowCharSet = 2023
	SFlowCSWindows31J              SFlowCharSet = 2024
	SFlowCSGB2312                  SFlowCharSet = 2025
	SFlowCSBig5                    SFlowCharSet = 2026
	SFlowCSMacintosh               SFlowCharSet = 2027
	SFlowCSIBM037                  SFlowCharSet = 2028
	SFlowCSIBM038                  SFlowCharSet = 2029
	SFlowCSIBM273                  SFlowCharSet = 2030
	SFlowCSIBM274                  SFlowCharSet = 2031
	SFlowCSIBM275                  SFlowCharSet = 2032
	SFlowCSIBM277                  SFlowCharSet = 2033
	SFlowCSIBM278                  SFlowCharSet = 2034
	SFlowCSIBM280                  SFlowCharSet = 2035
	SFlowCSIBM281                  SFlowCharSet = 2036
	SFlowCSIBM284                  SFlowCharSet = 2037
	SFlowCSIBM285                  SFlowCharSet = 2038
	SFlowCSIBM290                  SFlowCharSet = 2039
	SFlowCSIBM297                  SFlowCharSet = 2040
	SFlowCSIBM420                  SFlowCharSet = 2041
	SFlowCSIBM423                  SFlowCharSet = 2042
	SFlowCSIBM424                  SFlowCharSet = 2043
	SFlowCSIBM500                  SFlowCharSet = 2044
	SFlowCSIBM851                  SFlowCharSet = 2045
	SFlowCSIBM855                  SFlowCharSet = 2046
	SFlowCSIBM857                  SFlowCharSet = 2047
	SFlowCSIBM860                  SFlowCharSet = 2048
	SFlowCSIBM861                  SFlowCharSet = 2049
	SFlowCSIBM863                  SFlowCharSet = 2050
	SFlowCSIBM864                  SFlowCharSet = 2051
	SFlowCSIBM865                  SFlowCharSet = 2052
	SFlowCSIBM868                  SFlowCharSet = 2053
	SFlowCSIBM869                  SFlowCharSet = 2054
	SFlowCSIBM870                  SFlowCharSet = 2055
	SFlowCSIBM871                  SFlowCharSet = 2056
	SFlowCSIBM880                  SFlowCharSet = 2057
	SFlowCSIBM891                  SFlowCharSet = 2058
	SFlowCSIBM903                  SFlowCharSet = 2059
	SFlowCSIBBM904                 SFlowCharSet = 2060
	SFlowCSIBM905                  SFlowCharSet = 2061
	SFlowCSIBM918                  SFlowCharSet = 2062
	SFlowCSIBM1026                 SFlowCharSet = 2063
	SFlowCSIBMEBCDICATDE           SFlowCharSet = 2064
	SFlowCSEBCDICATDEA             SFlowCharSet = 2065
	SFlowCSEBCDICCAFR              SFlowCharSet = 2066
	SFlowCSEBCDICDKNO              SFlowCharSet = 2067
	SFlowCSEBCDICDKNOA             SFlowCharSet = 2068
	SFlowCSEBCDICFISE              SFlowCharSet = 2069
	SFlowCSEBCDICFISEA             SFlowCharSet = 2070
	SFlowCSEBCDICFR                SFlowCharSet = 2071
	SFlowCSEBCDICIT                SFlowCharSet = 2072
	SFlowCSEBCDICPT                SFlowCharSet = 2073
	SFlowCSEBCDICES                SFlowCharSet = 2074
	SFlowCSEBCDICESA               SFlowCharSet = 2075
	SFlowCSEBCDICESS               SFlowCharSet = 2076
	SFlowCSEBCDICUK                SFlowCharSet = 2077
	SFlowCSEBCDICUS                SFlowCharSet = 2078
	SFlowCSUnknown8BiT             SFlowCharSet = 2079
	SFlowCSMnemonic                SFlowCharSet = 2080
	SFlowCSMnem                    SFlowCharSet = 2081
	SFlowCSVISCII                  SFlowCharSet = 2082
	SFlowCSVIQR                    SFlowCharSet = 2083
	SFlowCSKOI8R                   SFlowCharSet = 2084
	SFlowCSHZGB2312                SFlowCharSet = 2085
	SFlowCSIBM866                  SFlowCharSet = 2086
	SFlowCSPC775Baltic             SFlowCharSet = 2087
	SFlowCSKOI8U                   SFlowCharSet = 2088
	SFlowCSIBM00858                SFlowCharSet = 2089
	SFlowCSIBM00924                SFlowCharSet = 2090
	SFlowCSIBM01140                SFlowCharSet = 2091
	SFlowCSIBM01141                SFlowCharSet = 2092
	SFlowCSIBM01142                SFlowCharSet = 2093
	SFlowCSIBM01143                SFlowCharSet = 2094
	SFlowCSIBM01144                SFlowCharSet = 2095
	SFlowCSIBM01145                SFlowCharSet = 2096
	SFlowCSIBM01146                SFlowCharSet = 2097
	SFlowCSIBM01147                SFlowCharSet = 2098
	SFlowCSIBM01148                SFlowCharSet = 2099
	SFlowCSIBM01149                SFlowCharSet = 2100
	SFlowCSBig5HKSCS               SFlowCharSet = 2101
	SFlowCSIBM1047                 SFlowCharSet = 2102
	SFlowCSPTCP154                 SFlowCharSet = 2103
	SFlowCSAmiga1251               SFlowCharSet = 2104
	SFlowCSKOI7switched            SFlowCharSet = 2105
	SFlowCSBRF                     SFlowCharSet = 2106
	SFlowCSTSCII                   SFlowCharSet = 2107
	SFlowCSCP51932                 SFlowCharSet = 2108
	SFlowCSWindows874              SFlowCharSet = 2109
	SFlowCSWindows1250             SFlowCharSet = 2250
	SFlowCSWindows1251             SFlowCharSet = 2251
	SFlowCSWindows1252             SFlowCharSet = 2252
	SFlowCSWindows1253             SFlowCharSet = 2253
	SFlowCSWindows1254             SFlowCharSet = 2254
	SFlowCSWindows1255             SFlowCharSet = 2255
	SFlowCSWindows1256             SFlowCharSet = 2256
	SFlowCSWindows1257             SFlowCharSet = 2257
	SFlowCSWindows1258             SFlowCharSet = 2258
	SFlowCSTIS620                  SFlowCharSet = 2259
	SFlowCS50220                   SFlowCharSet = 2260
	SFlowCSreserved                SFlowCharSet = 3000
)

func decodeExtendedUserFlow(data *[]byte) (SFlowExtendedUserFlow, error) {
	eu := SFlowExtendedUserFlow{}
	var fdf SFlowFlowDataFormat
	var srcUserLen uint32
	var srcUserLenWithPad int
	var srcUserBytes []byte
	var dstUserLen uint32
	var dstUserLenWithPad int
	var dstUserBytes []byte

	*data, fdf = (*data)[4:], SFlowFlowDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	eu.EnterpriseID, eu.Format = fdf.decode()
	*data, eu.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, eu.SourceCharSet = (*data)[4:], SFlowCharSet(binary.BigEndian.Uint32((*data)[:4]))
	*data, srcUserLen = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	srcUserLenWithPad = int(srcUserLen + ((4 - srcUserLen) % 4))
	*data, srcUserBytes = (*data)[srcUserLenWithPad:], (*data)[:srcUserLenWithPad]
	eu.SourceUserID = string(srcUserBytes[:srcUserLen])
	*data, eu.DestinationCharSet = (*data)[4:], SFlowCharSet(binary.BigEndian.Uint32((*data)[:4]))
	*data, dstUserLen = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	dstUserLenWithPad = int(dstUserLen + ((4 - dstUserLen) % 4))
	*data, dstUserBytes = (*data)[dstUserLenWithPad:], (*data)[:dstUserLenWithPad]
	eu.DestinationUserID = string(dstUserBytes[:dstUserLen])
	return eu, nil
}

// **************************************************
//  Packet IP version 4 Record
// **************************************************

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                     Length                    |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                    Protocol                   |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  Source IPv4                  |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                Destination IPv4               |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                   Source Port                 |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                Destionation Port              |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                   TCP Flags                   |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                      TOS                      |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
type SFlowIpv4Record struct {
	// The length of the IP packet excluding ower layer encapsulations
	Length uint32
	// IP Protocol type (for example, TCP = 6, UDP = 17)
	Protocol uint32
	// Source IP Address
	IPSrc net.IP
	// Destination IP Address
	IPDst net.IP
	// TCP/UDP source port number or equivalent
	PortSrc uint32
	// TCP/UDP destination port number or equivalent
	PortDst uint32
	// TCP flags
	TCPFlags uint32
	// IP type of service
	TOS uint32
}

func decodeSFlowIpv4Record(data *[]byte) (SFlowIpv4Record, error) {
	si := SFlowIpv4Record{}

	*data, si.Length = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, si.Protocol = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, si.IPSrc = (*data)[4:], net.IP((*data)[:4])
	*data, si.IPDst = (*data)[4:], net.IP((*data)[:4])
	*data, si.PortSrc = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, si.PortDst = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, si.TCPFlags = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, si.TOS = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])

	return si, nil
}

// **************************************************
//  Packet IP version 6 Record
// **************************************************

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                     Length                    |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                    Protocol                   |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  Source IPv4                  |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                Destination IPv4               |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                   Source Port                 |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                Destionation Port              |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                   TCP Flags                   |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                    Priority                   |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
type SFlowIpv6Record struct {
	// The length of the IP packet excluding ower layer encapsulations
	Length uint32
	// IP Protocol type (for example, TCP = 6, UDP = 17)
	Protocol uint32
	// Source IP Address
	IPSrc net.IP
	// Destination IP Address
	IPDst net.IP
	// TCP/UDP source port number or equivalent
	PortSrc uint32
	// TCP/UDP destination port number or equivalent
	PortDst uint32
	// TCP flags
	TCPFlags uint32
	// IP priority
	Priority uint32
}

func decodeSFlowIpv6Record(data *[]byte) (SFlowIpv6Record, error) {
	si := SFlowIpv6Record{}

	*data, si.Length = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, si.Protocol = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, si.IPSrc = (*data)[16:], net.IP((*data)[:16])
	*data, si.IPDst = (*data)[16:], net.IP((*data)[:16])
	*data, si.PortSrc = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, si.PortDst = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, si.TCPFlags = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, si.Priority = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])

	return si, nil
}

// **************************************************
//  Extended IPv4 Tunnel Egress
// **************************************************

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  record length                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  /           Packet IP version 4 Record          /
//  /                                               /
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
type SFlowExtendedIpv4TunnelEgressRecord struct {
	SFlowBaseFlowRecord
	SFlowIpv4Record SFlowIpv4Record
}

func decodeExtendedIpv4TunnelEgress(data *[]byte) (SFlowExtendedIpv4TunnelEgressRecord, error) {
	rec := SFlowExtendedIpv4TunnelEgressRecord{}
	var fdf SFlowFlowDataFormat

	*data, fdf = (*data)[4:], SFlowFlowDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	rec.EnterpriseID, rec.Format = fdf.decode()
	*data, rec.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	rec.SFlowIpv4Record, _ = decodeSFlowIpv4Record(data)

	return rec, nil
}

// **************************************************
//  Extended IPv4 Tunnel Ingress
// **************************************************

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  record length                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  /           Packet IP version 4 Record          /
//  /                                               /
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
type SFlowExtendedIpv4TunnelIngressRecord struct {
	SFlowBaseFlowRecord
	SFlowIpv4Record SFlowIpv4Record
}

func decodeExtendedIpv4TunnelIngress(data *[]byte) (SFlowExtendedIpv4TunnelIngressRecord, error) {
	rec := SFlowExtendedIpv4TunnelIngressRecord{}
	var fdf SFlowFlowDataFormat

	*data, fdf = (*data)[4:], SFlowFlowDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	rec.EnterpriseID, rec.Format = fdf.decode()
	*data, rec.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	rec.SFlowIpv4Record, _ = decodeSFlowIpv4Record(data)

	return rec, nil
}

// **************************************************
//  Extended IPv6 Tunnel Egress
// **************************************************

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  record length                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  /           Packet IP version 6 Record          /
//  /                                               /
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
type SFlowExtendedIpv6TunnelEgressRecord struct {
	SFlowBaseFlowRecord
	SFlowIpv6Record
}

func decodeExtendedIpv6TunnelEgress(data *[]byte) (SFlowExtendedIpv6TunnelEgressRecord, error) {
	rec := SFlowExtendedIpv6TunnelEgressRecord{}
	var fdf SFlowFlowDataFormat

	*data, fdf = (*data)[4:], SFlowFlowDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	rec.EnterpriseID, rec.Format = fdf.decode()
	*data, rec.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	rec.SFlowIpv6Record, _ = decodeSFlowIpv6Record(data)

	return rec, nil
}

// **************************************************
//  Extended IPv6 Tunnel Ingress
// **************************************************

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  record length                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  /           Packet IP version 6 Record          /
//  /                                               /
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
type SFlowExtendedIpv6TunnelIngressRecord struct {
	SFlowBaseFlowRecord
	SFlowIpv6Record
}

func decodeExtendedIpv6TunnelIngress(data *[]byte) (SFlowExtendedIpv6TunnelIngressRecord, error) {
	rec := SFlowExtendedIpv6TunnelIngressRecord{}
	var fdf SFlowFlowDataFormat

	*data, fdf = (*data)[4:], SFlowFlowDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	rec.EnterpriseID, rec.Format = fdf.decode()
	*data, rec.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	rec.SFlowIpv6Record, _ = decodeSFlowIpv6Record(data)

	return rec, nil
}

// **************************************************
//  Extended Decapsulate Egress
// **************************************************

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  record length                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |               Inner Header Offset             |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
type SFlowExtendedDecapsulateEgressRecord struct {
	SFlowBaseFlowRecord
	InnerHeaderOffset uint32
}

func decodeExtendedDecapsulateEgress(data *[]byte) (SFlowExtendedDecapsulateEgressRecord, error) {
	rec := SFlowExtendedDecapsulateEgressRecord{}
	var fdf SFlowFlowDataFormat

	*data, fdf = (*data)[4:], SFlowFlowDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	rec.EnterpriseID, rec.Format = fdf.decode()
	*data, rec.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, rec.InnerHeaderOffset = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])

	return rec, nil
}

// **************************************************
//  Extended Decapsulate Ingress
// **************************************************

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  record length                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |               Inner Header Offset             |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
type SFlowExtendedDecapsulateIngressRecord struct {
	SFlowBaseFlowRecord
	InnerHeaderOffset uint32
}

func decodeExtendedDecapsulateIngress(data *[]byte) (SFlowExtendedDecapsulateIngressRecord, error) {
	rec := SFlowExtendedDecapsulateIngressRecord{}
	var fdf SFlowFlowDataFormat

	*data, fdf = (*data)[4:], SFlowFlowDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	rec.EnterpriseID, rec.Format = fdf.decode()
	*data, rec.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, rec.InnerHeaderOffset = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])

	return rec, nil
}

// **************************************************
//  Extended VNI Egress
// **************************************************

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  record length                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                       VNI                     |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
type SFlowExtendedVniEgressRecord struct {
	SFlowBaseFlowRecord
	VNI uint32
}

func decodeExtendedVniEgress(data *[]byte) (SFlowExtendedVniEgressRecord, error) {
	rec := SFlowExtendedVniEgressRecord{}
	var fdf SFlowFlowDataFormat

	*data, fdf = (*data)[4:], SFlowFlowDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	rec.EnterpriseID, rec.Format = fdf.decode()
	*data, rec.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, rec.VNI = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])

	return rec, nil
}

// **************************************************
//  Extended VNI Ingress
// **************************************************

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  record length                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                       VNI                     |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
type SFlowExtendedVniIngressRecord struct {
	SFlowBaseFlowRecord
	VNI uint32
}

func decodeExtendedVniIngress(data *[]byte) (SFlowExtendedVniIngressRecord, error) {
	rec := SFlowExtendedVniIngressRecord{}
	var fdf SFlowFlowDataFormat

	*data, fdf = (*data)[4:], SFlowFlowDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	rec.EnterpriseID, rec.Format = fdf.decode()
	*data, rec.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, rec.VNI = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])

	return rec, nil
}

// **************************************************
//  Counter Record
// **************************************************

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  counter length               |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  /                   counter data                /
//  /                                               /
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

type SFlowBaseCounterRecord struct {
	EnterpriseID   SFlowEnterpriseID
	Format         SFlowCounterRecordType
	FlowDataLength uint32
}

func (bcr SFlowBaseCounterRecord) GetType() SFlowCounterRecordType {
	switch bcr.Format {
	case SFlowTypeGenericInterfaceCounters:
		return SFlowTypeGenericInterfaceCounters
	case SFlowTypeEthernetInterfaceCounters:
		return SFlowTypeEthernetInterfaceCounters
	case SFlowTypeTokenRingInterfaceCounters:
		return SFlowTypeTokenRingInterfaceCounters
	case SFlowType100BaseVGInterfaceCounters:
		return SFlowType100BaseVGInterfaceCounters
	case SFlowTypeVLANCounters:
		return SFlowTypeVLANCounters
	case SFlowTypeLACPCounters:
		return SFlowTypeLACPCounters
	case SFlowTypeProcessorCounters:
		return SFlowTypeProcessorCounters
	case SFlowTypeOpenflowPortCounters:
		return SFlowTypeOpenflowPortCounters
	case SFlowTypePORTNAMECounters:
		return SFlowTypePORTNAMECounters
	case SFLowTypeAPPRESOURCESCounters:
		return SFLowTypeAPPRESOURCESCounters
	case SFlowTypeOVSDPCounters:
		return SFlowTypeOVSDPCounters
	}
	unrecognized := fmt.Sprint("Unrecognized counter record type:", bcr.Format)
	panic(unrecognized)
}

// **************************************************
//  Counter Record
// **************************************************

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  counter length               |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                    IfIndex                    |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                    IfType                     |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                   IfSpeed                     |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                   IfDirection                 |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                    IfStatus                   |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                   IFInOctets                  |
//  |                                               |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                   IfInUcastPkts               |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  IfInMulticastPkts            |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  IfInBroadcastPkts            |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                    IfInDiscards               |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                    InInErrors                 |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  IfInUnknownProtos            |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                   IfOutOctets                 |
//  |                                               |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                   IfOutUcastPkts              |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  IfOutMulticastPkts           |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  IfOutBroadcastPkts           |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                   IfOutDiscards               |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                    IfOUtErrors                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                 IfPromiscouousMode            |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

type SFlowGenericInterfaceCounters struct {
	SFlowBaseCounterRecord
	IfIndex            uint32
	IfType             uint32
	IfSpeed            uint64
	IfDirection        uint32
	IfStatus           uint32
	IfInOctets         uint64
	IfInUcastPkts      uint32
	IfInMulticastPkts  uint32
	IfInBroadcastPkts  uint32
	IfInDiscards       uint32
	IfInErrors         uint32
	IfInUnknownProtos  uint32
	IfOutOctets        uint64
	IfOutUcastPkts     uint32
	IfOutMulticastPkts uint32
	IfOutBroadcastPkts uint32
	IfOutDiscards      uint32
	IfOutErrors        uint32
	IfPromiscuousMode  uint32
}

func decodeGenericInterfaceCounters(data *[]byte) (SFlowGenericInterfaceCounters, error) {
	gic := SFlowGenericInterfaceCounters{}
	var cdf SFlowCounterDataFormat

	*data, cdf = (*data)[4:], SFlowCounterDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	gic.EnterpriseID, gic.Format = cdf.decode()
	*data, gic.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, gic.IfIndex = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, gic.IfType = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, gic.IfSpeed = (*data)[8:], binary.BigEndian.Uint64((*data)[:8])
	*data, gic.IfDirection = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, gic.IfStatus = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, gic.IfInOctets = (*data)[8:], binary.BigEndian.Uint64((*data)[:8])
	*data, gic.IfInUcastPkts = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, gic.IfInMulticastPkts = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, gic.IfInBroadcastPkts = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, gic.IfInDiscards = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, gic.IfInErrors = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, gic.IfInUnknownProtos = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, gic.IfOutOctets = (*data)[8:], binary.BigEndian.Uint64((*data)[:8])
	*data, gic.IfOutUcastPkts = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, gic.IfOutMulticastPkts = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, gic.IfOutBroadcastPkts = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, gic.IfOutDiscards = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, gic.IfOutErrors = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, gic.IfPromiscuousMode = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	return gic, nil
}

// **************************************************
//  Counter Record
// **************************************************

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  counter length               |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  /                   counter data                /
//  /                                               /
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

type SFlowEthernetCounters struct {
	SFlowBaseCounterRecord
	AlignmentErrors           uint32
	FCSErrors                 uint32
	SingleCollisionFrames     uint32
	MultipleCollisionFrames   uint32
	SQETestErrors             uint32
	DeferredTransmissions     uint32
	LateCollisions            uint32
	ExcessiveCollisions       uint32
	InternalMacTransmitErrors uint32
	CarrierSenseErrors        uint32
	FrameTooLongs             uint32
	InternalMacReceiveErrors  uint32
	SymbolErrors              uint32
}

func decodeEthernetCounters(data *[]byte) (SFlowEthernetCounters, error) {
	ec := SFlowEthernetCounters{}
	var cdf SFlowCounterDataFormat

	*data, cdf = (*data)[4:], SFlowCounterDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	ec.EnterpriseID, ec.Format = cdf.decode()
	if len(*data) < 4 {
		return SFlowEthernetCounters{}, errors.New("ethernet counters too small")
	}
	*data, ec.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	if len(*data) < 4 {
		return SFlowEthernetCounters{}, errors.New("ethernet counters too small")
	}
	*data, ec.AlignmentErrors = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	if len(*data) < 4 {
		return SFlowEthernetCounters{}, errors.New("ethernet counters too small")
	}
	*data, ec.FCSErrors = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	if len(*data) < 4 {
		return SFlowEthernetCounters{}, errors.New("ethernet counters too small")
	}
	*data, ec.SingleCollisionFrames = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	if len(*data) < 4 {
		return SFlowEthernetCounters{}, errors.New("ethernet counters too small")
	}
	*data, ec.MultipleCollisionFrames = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	if len(*data) < 4 {
		return SFlowEthernetCounters{}, errors.New("ethernet counters too small")
	}
	*data, ec.SQETestErrors = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	if len(*data) < 4 {
		return SFlowEthernetCounters{}, errors.New("ethernet counters too small")
	}
	*data, ec.DeferredTransmissions = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	if len(*data) < 4 {
		return SFlowEthernetCounters{}, errors.New("ethernet counters too small")
	}
	*data, ec.LateCollisions = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	if len(*data) < 4 {
		return SFlowEthernetCounters{}, errors.New("ethernet counters too small")
	}
	*data, ec.ExcessiveCollisions = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	if len(*data) < 4 {
		return SFlowEthernetCounters{}, errors.New("ethernet counters too small")
	}
	*data, ec.InternalMacTransmitErrors = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	if len(*data) < 4 {
		return SFlowEthernetCounters{}, errors.New("ethernet counters too small")
	}
	*data, ec.CarrierSenseErrors = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	if len(*data) < 4 {
		return SFlowEthernetCounters{}, errors.New("ethernet counters too small")
	}
	*data, ec.FrameTooLongs = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	if len(*data) < 4 {
		return SFlowEthernetCounters{}, errors.New("ethernet counters too small")
	}
	*data, ec.InternalMacReceiveErrors = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	if len(*data) < 4 {
		return SFlowEthernetCounters{}, errors.New("ethernet counters too small")
	}
	*data, ec.SymbolErrors = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	return ec, nil
}

// VLAN Counter

type SFlowVLANCounters struct {
	SFlowBaseCounterRecord
	VlanID        uint32
	Octets        uint64
	UcastPkts     uint32
	MulticastPkts uint32
	BroadcastPkts uint32
	Discards      uint32
}

func decodeVLANCounters(data *[]byte) (SFlowVLANCounters, error) {
	vc := SFlowVLANCounters{}
	var cdf SFlowCounterDataFormat

	*data, cdf = (*data)[4:], SFlowCounterDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	vc.EnterpriseID, vc.Format = cdf.decode()
	vc.EnterpriseID, vc.Format = cdf.decode()
	*data, vc.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, vc.VlanID = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, vc.Octets = (*data)[8:], binary.BigEndian.Uint64((*data)[:8])
	*data, vc.UcastPkts = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, vc.MulticastPkts = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, vc.BroadcastPkts = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, vc.Discards = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	return vc, nil
}

//SFLLACPportState  :  SFlow LACP Port State (All(4) - 32 bit)
type SFLLACPPortState struct {
	PortStateAll uint32
}

//LACPcounters  :  LACP SFlow Counters  ( 64 Bytes )
type SFlowLACPCounters struct {
	SFlowBaseCounterRecord
	ActorSystemID        net.HardwareAddr
	PartnerSystemID      net.HardwareAddr
	AttachedAggID        uint32
	LacpPortState        SFLLACPPortState
	LACPDUsRx            uint32
	MarkerPDUsRx         uint32
	MarkerResponsePDUsRx uint32
	UnknownRx            uint32
	IllegalRx            uint32
	LACPDUsTx            uint32
	MarkerPDUsTx         uint32
	MarkerResponsePDUsTx uint32
}

func decodeLACPCounters(data *[]byte) (SFlowLACPCounters, error) {
	la := SFlowLACPCounters{}
	var cdf SFlowCounterDataFormat

	*data, cdf = (*data)[4:], SFlowCounterDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	la.EnterpriseID, la.Format = cdf.decode()
	*data, la.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, la.ActorSystemID = (*data)[6:], (*data)[:6]
	*data = (*data)[2:] // remove padding
	*data, la.PartnerSystemID = (*data)[6:], (*data)[:6]
	*data = (*data)[2:] //remove padding
	*data, la.AttachedAggID = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, la.LacpPortState.PortStateAll = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, la.LACPDUsRx = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, la.MarkerPDUsRx = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, la.MarkerResponsePDUsRx = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, la.UnknownRx = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, la.IllegalRx = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, la.LACPDUsTx = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, la.MarkerPDUsTx = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, la.MarkerResponsePDUsTx = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])

	return la, nil

}

// **************************************************
//  Processor Counter Record
// **************************************************
//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  counter length               |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                    FiveSecCpu                 |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                    OneMinCpu                  |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                    GiveMinCpu                 |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                   TotalMemory                 |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                    FreeMemory                 |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

type SFlowProcessorCounters struct {
	SFlowBaseCounterRecord
	FiveSecCpu  uint32 // 5 second average CPU utilization
	OneMinCpu   uint32 // 1 minute average CPU utilization
	FiveMinCpu  uint32 // 5 minute average CPU utilization
	TotalMemory uint64 // total memory (in bytes)
	FreeMemory  uint64 // free memory (in bytes)
}

func decodeProcessorCounters(data *[]byte) (SFlowProcessorCounters, error) {
	pc := SFlowProcessorCounters{}
	var cdf SFlowCounterDataFormat
	var high32, low32 uint32

	*data, cdf = (*data)[4:], SFlowCounterDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	pc.EnterpriseID, pc.Format = cdf.decode()
	*data, pc.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])

	*data, pc.FiveSecCpu = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, pc.OneMinCpu = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, pc.FiveMinCpu = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, high32 = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, low32 = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	pc.TotalMemory = (uint64(high32) << 32) + uint64(low32)
	*data, high32 = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, low32 = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	pc.FreeMemory = (uint64(high32)) + uint64(low32)

	return pc, nil
}

// SFlowEthernetFrameFlowRecord give additional information
// about the sampled packet if it's available.
// An agent may or may not provide this information.
type SFlowEthernetFrameFlowRecord struct {
	SFlowBaseFlowRecord
	FrameLength uint32
	SrcMac      net.HardwareAddr
	DstMac      net.HardwareAddr
	Type        uint32
}

// Ethernet frame flow records have the following structure:

//  0                      15                      31
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |      20 bit Interprise (0)     |12 bit format |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                  record length                |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |                Source Mac Address             |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |             Destination Mac Address           |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//  |               Ethernet Packet Type            |
//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

func decodeEthernetFrameFlowRecord(data *[]byte) (SFlowEthernetFrameFlowRecord, error) {
	es := SFlowEthernetFrameFlowRecord{}
	var fdf SFlowFlowDataFormat

	*data, fdf = (*data)[4:], SFlowFlowDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	es.EnterpriseID, es.Format = fdf.decode()
	*data, es.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])

	*data, es.FrameLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, es.SrcMac = (*data)[8:], net.HardwareAddr((*data)[:6])
	*data, es.DstMac = (*data)[8:], net.HardwareAddr((*data)[:6])
	*data, es.Type = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	return es, nil
}

//SFlowOpenflowPortCounters  :  OVS-Sflow OpenFlow Port Counter  ( 20 Bytes )
type SFlowOpenflowPortCounters struct {
	SFlowBaseCounterRecord
	DatapathID uint64
	PortNo     uint32
}

func decodeOpenflowportCounters(data *[]byte) (SFlowOpenflowPortCounters, error) {
	ofp := SFlowOpenflowPortCounters{}
	var cdf SFlowCounterDataFormat

	*data, cdf = (*data)[4:], SFlowCounterDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	ofp.EnterpriseID, ofp.Format = cdf.decode()
	*data, ofp.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, ofp.DatapathID = (*data)[8:], binary.BigEndian.Uint64((*data)[:8])
	*data, ofp.PortNo = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])

	return ofp, nil
}

//SFlowAppresourcesCounters  :  OVS_Sflow App Resources Counter ( 48 Bytes )
type SFlowAppresourcesCounters struct {
	SFlowBaseCounterRecord
	UserTime   uint32
	SystemTime uint32
	MemUsed    uint64
	MemMax     uint64
	FdOpen     uint32
	FdMax      uint32
	ConnOpen   uint32
	ConnMax    uint32
}

func decodeAppresourcesCounters(data *[]byte) (SFlowAppresourcesCounters, error) {
	app := SFlowAppresourcesCounters{}
	var cdf SFlowCounterDataFormat

	*data, cdf = (*data)[4:], SFlowCounterDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	app.EnterpriseID, app.Format = cdf.decode()
	*data, app.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, app.UserTime = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, app.SystemTime = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, app.MemUsed = (*data)[8:], binary.BigEndian.Uint64((*data)[:8])
	*data, app.MemMax = (*data)[8:], binary.BigEndian.Uint64((*data)[:8])
	*data, app.FdOpen = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, app.FdMax = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, app.ConnOpen = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, app.ConnMax = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])

	return app, nil
}

//SFlowOVSDPCounters  :  OVS-Sflow DataPath Counter  ( 32 Bytes )
type SFlowOVSDPCounters struct {
	SFlowBaseCounterRecord
	NHit     uint32
	NMissed  uint32
	NLost    uint32
	NMaskHit uint32
	NFlows   uint32
	NMasks   uint32
}

func decodeOVSDPCounters(data *[]byte) (SFlowOVSDPCounters, error) {
	dp := SFlowOVSDPCounters{}
	var cdf SFlowCounterDataFormat

	*data, cdf = (*data)[4:], SFlowCounterDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	dp.EnterpriseID, dp.Format = cdf.decode()
	*data, dp.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, dp.NHit = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, dp.NMissed = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, dp.NLost = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, dp.NMaskHit = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, dp.NFlows = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	*data, dp.NMasks = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])

	return dp, nil
}

//SFlowPORTNAME  :  OVS-Sflow PORTNAME Counter Sampletype ( 20 Bytes )
type SFlowPORTNAME struct {
	SFlowBaseCounterRecord
	Len uint32
	Str string
}

func decodeString(data *[]byte) (len uint32, str string) {
	*data, len = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	str = string((*data)[:len])
	if (len % 4) != 0 {
		len += 4 - len%4
	}
	*data = (*data)[len:]
	return
}

func decodePortnameCounters(data *[]byte) (SFlowPORTNAME, error) {
	pn := SFlowPORTNAME{}
	var cdf SFlowCounterDataFormat

	*data, cdf = (*data)[4:], SFlowCounterDataFormat(binary.BigEndian.Uint32((*data)[:4]))
	pn.EnterpriseID, pn.Format = cdf.decode()
	*data, pn.FlowDataLength = (*data)[4:], binary.BigEndian.Uint32((*data)[:4])
	pn.Len, pn.Str = decodeString(data)

	return pn, nil
}
