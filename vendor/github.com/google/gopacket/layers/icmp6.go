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
	"fmt"
	"reflect"

	"github.com/google/gopacket"
)

const (
	// The following are from RFC 4443
	ICMPv6TypeDestinationUnreachable = 1
	ICMPv6TypePacketTooBig           = 2
	ICMPv6TypeTimeExceeded           = 3
	ICMPv6TypeParameterProblem       = 4
	ICMPv6TypeEchoRequest            = 128
	ICMPv6TypeEchoReply              = 129

	// The following are from RFC 4861
	ICMPv6TypeRouterSolicitation    = 133
	ICMPv6TypeRouterAdvertisement   = 134
	ICMPv6TypeNeighborSolicitation  = 135
	ICMPv6TypeNeighborAdvertisement = 136
	ICMPv6TypeRedirect              = 137

	// The following are from RFC 2710
	ICMPv6TypeMLDv1MulticastListenerQueryMessage  = 130
	ICMPv6TypeMLDv1MulticastListenerReportMessage = 131
	ICMPv6TypeMLDv1MulticastListenerDoneMessage   = 132

	// The following are from RFC 3810
	ICMPv6TypeMLDv2MulticastListenerReportMessageV2 = 143
)

const (
	// DestinationUnreachable
	ICMPv6CodeNoRouteToDst           = 0
	ICMPv6CodeAdminProhibited        = 1
	ICMPv6CodeBeyondScopeOfSrc       = 2
	ICMPv6CodeAddressUnreachable     = 3
	ICMPv6CodePortUnreachable        = 4
	ICMPv6CodeSrcAddressFailedPolicy = 5
	ICMPv6CodeRejectRouteToDst       = 6

	// TimeExceeded
	ICMPv6CodeHopLimitExceeded               = 0
	ICMPv6CodeFragmentReassemblyTimeExceeded = 1

	// ParameterProblem
	ICMPv6CodeErroneousHeaderField   = 0
	ICMPv6CodeUnrecognizedNextHeader = 1
	ICMPv6CodeUnrecognizedIPv6Option = 2
)

type icmpv6TypeCodeInfoStruct struct {
	typeStr string
	codeStr *map[uint8]string
}

var (
	icmpv6TypeCodeInfo = map[uint8]icmpv6TypeCodeInfoStruct{
		ICMPv6TypeDestinationUnreachable: icmpv6TypeCodeInfoStruct{
			"DestinationUnreachable", &map[uint8]string{
				ICMPv6CodeNoRouteToDst:           "NoRouteToDst",
				ICMPv6CodeAdminProhibited:        "AdminProhibited",
				ICMPv6CodeBeyondScopeOfSrc:       "BeyondScopeOfSrc",
				ICMPv6CodeAddressUnreachable:     "AddressUnreachable",
				ICMPv6CodePortUnreachable:        "PortUnreachable",
				ICMPv6CodeSrcAddressFailedPolicy: "SrcAddressFailedPolicy",
				ICMPv6CodeRejectRouteToDst:       "RejectRouteToDst",
			},
		},
		ICMPv6TypePacketTooBig: icmpv6TypeCodeInfoStruct{
			"PacketTooBig", nil,
		},
		ICMPv6TypeTimeExceeded: icmpv6TypeCodeInfoStruct{
			"TimeExceeded", &map[uint8]string{
				ICMPv6CodeHopLimitExceeded:               "HopLimitExceeded",
				ICMPv6CodeFragmentReassemblyTimeExceeded: "FragmentReassemblyTimeExceeded",
			},
		},
		ICMPv6TypeParameterProblem: icmpv6TypeCodeInfoStruct{
			"ParameterProblem", &map[uint8]string{
				ICMPv6CodeErroneousHeaderField:   "ErroneousHeaderField",
				ICMPv6CodeUnrecognizedNextHeader: "UnrecognizedNextHeader",
				ICMPv6CodeUnrecognizedIPv6Option: "UnrecognizedIPv6Option",
			},
		},
		ICMPv6TypeEchoRequest: icmpv6TypeCodeInfoStruct{
			"EchoRequest", nil,
		},
		ICMPv6TypeEchoReply: icmpv6TypeCodeInfoStruct{
			"EchoReply", nil,
		},
		ICMPv6TypeRouterSolicitation: icmpv6TypeCodeInfoStruct{
			"RouterSolicitation", nil,
		},
		ICMPv6TypeRouterAdvertisement: icmpv6TypeCodeInfoStruct{
			"RouterAdvertisement", nil,
		},
		ICMPv6TypeNeighborSolicitation: icmpv6TypeCodeInfoStruct{
			"NeighborSolicitation", nil,
		},
		ICMPv6TypeNeighborAdvertisement: icmpv6TypeCodeInfoStruct{
			"NeighborAdvertisement", nil,
		},
		ICMPv6TypeRedirect: icmpv6TypeCodeInfoStruct{
			"Redirect", nil,
		},
	}
)

type ICMPv6TypeCode uint16

// Type returns the ICMPv6 type field.
func (a ICMPv6TypeCode) Type() uint8 {
	return uint8(a >> 8)
}

// Code returns the ICMPv6 code field.
func (a ICMPv6TypeCode) Code() uint8 {
	return uint8(a)
}

func (a ICMPv6TypeCode) String() string {
	t, c := a.Type(), a.Code()
	strInfo, ok := icmpv6TypeCodeInfo[t]
	if !ok {
		// Unknown ICMPv6 type field
		return fmt.Sprintf("%d(%d)", t, c)
	}
	typeStr := strInfo.typeStr
	if strInfo.codeStr == nil && c == 0 {
		// The ICMPv6 type does not make use of the code field
		return fmt.Sprintf("%s", strInfo.typeStr)
	}
	if strInfo.codeStr == nil && c != 0 {
		// The ICMPv6 type does not make use of the code field, but it is present anyway
		return fmt.Sprintf("%s(Code: %d)", typeStr, c)
	}
	codeStr, ok := (*strInfo.codeStr)[c]
	if !ok {
		// We don't know this ICMPv6 code; print the numerical value
		return fmt.Sprintf("%s(Code: %d)", typeStr, c)
	}
	return fmt.Sprintf("%s(%s)", typeStr, codeStr)
}

func (a ICMPv6TypeCode) GoString() string {
	t := reflect.TypeOf(a)
	return fmt.Sprintf("%s(%d, %d)", t.String(), a.Type(), a.Code())
}

// SerializeTo writes the ICMPv6TypeCode value to the 'bytes' buffer.
func (a ICMPv6TypeCode) SerializeTo(bytes []byte) {
	binary.BigEndian.PutUint16(bytes, uint16(a))
}

// CreateICMPv6TypeCode is a convenience function to create an ICMPv6TypeCode
// gopacket type from the ICMPv6 type and code values.
func CreateICMPv6TypeCode(typ uint8, code uint8) ICMPv6TypeCode {
	return ICMPv6TypeCode(binary.BigEndian.Uint16([]byte{typ, code}))
}

// ICMPv6 is the layer for IPv6 ICMP packet data
type ICMPv6 struct {
	BaseLayer
	TypeCode ICMPv6TypeCode
	Checksum uint16
	// TypeBytes is deprecated and always nil. See the different ICMPv6 message types
	// instead (e.g. ICMPv6TypeRouterSolicitation).
	TypeBytes []byte
	tcpipchecksum
}

// LayerType returns LayerTypeICMPv6.
func (i *ICMPv6) LayerType() gopacket.LayerType { return LayerTypeICMPv6 }

// DecodeFromBytes decodes the given bytes into this layer.
func (i *ICMPv6) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 4 {
		df.SetTruncated()
		return errors.New("ICMP layer less then 4 bytes for ICMPv6 packet")
	}
	i.TypeCode = CreateICMPv6TypeCode(data[0], data[1])
	i.Checksum = binary.BigEndian.Uint16(data[2:4])
	i.BaseLayer = BaseLayer{data[:4], data[4:]}
	return nil
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (i *ICMPv6) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	bytes, err := b.PrependBytes(4)
	if err != nil {
		return err
	}
	i.TypeCode.SerializeTo(bytes)

	if opts.ComputeChecksums {
		bytes[2] = 0
		bytes[3] = 0
		csum, err := i.computeChecksum(b.Bytes(), IPProtocolICMPv6)
		if err != nil {
			return err
		}
		i.Checksum = csum
	}
	binary.BigEndian.PutUint16(bytes[2:], i.Checksum)

	return nil
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (i *ICMPv6) CanDecode() gopacket.LayerClass {
	return LayerTypeICMPv6
}

// NextLayerType returns the layer type contained by this DecodingLayer.
func (i *ICMPv6) NextLayerType() gopacket.LayerType {
	switch i.TypeCode.Type() {
	case ICMPv6TypeEchoRequest:
		return LayerTypeICMPv6Echo
	case ICMPv6TypeEchoReply:
		return LayerTypeICMPv6Echo
	case ICMPv6TypeRouterSolicitation:
		return LayerTypeICMPv6RouterSolicitation
	case ICMPv6TypeRouterAdvertisement:
		return LayerTypeICMPv6RouterAdvertisement
	case ICMPv6TypeNeighborSolicitation:
		return LayerTypeICMPv6NeighborSolicitation
	case ICMPv6TypeNeighborAdvertisement:
		return LayerTypeICMPv6NeighborAdvertisement
	case ICMPv6TypeRedirect:
		return LayerTypeICMPv6Redirect
	case ICMPv6TypeMLDv1MulticastListenerQueryMessage: // Same Code for MLDv1 Query and MLDv2 Query
		if len(i.Payload) > 20 { // Only payload size differs
			return LayerTypeMLDv2MulticastListenerQuery
		} else {
			return LayerTypeMLDv1MulticastListenerQuery
		}
	case ICMPv6TypeMLDv1MulticastListenerDoneMessage:
		return LayerTypeMLDv1MulticastListenerDone
	case ICMPv6TypeMLDv1MulticastListenerReportMessage:
		return LayerTypeMLDv1MulticastListenerReport
	case ICMPv6TypeMLDv2MulticastListenerReportMessageV2:
		return LayerTypeMLDv2MulticastListenerReport
	}

	return gopacket.LayerTypePayload
}

func decodeICMPv6(data []byte, p gopacket.PacketBuilder) error {
	i := &ICMPv6{}
	return decodingLayerDecoder(i, data, p)
}
