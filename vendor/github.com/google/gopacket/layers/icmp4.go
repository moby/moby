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
	ICMPv4TypeEchoReply              = 0
	ICMPv4TypeDestinationUnreachable = 3
	ICMPv4TypeSourceQuench           = 4
	ICMPv4TypeRedirect               = 5
	ICMPv4TypeEchoRequest            = 8
	ICMPv4TypeRouterAdvertisement    = 9
	ICMPv4TypeRouterSolicitation     = 10
	ICMPv4TypeTimeExceeded           = 11
	ICMPv4TypeParameterProblem       = 12
	ICMPv4TypeTimestampRequest       = 13
	ICMPv4TypeTimestampReply         = 14
	ICMPv4TypeInfoRequest            = 15
	ICMPv4TypeInfoReply              = 16
	ICMPv4TypeAddressMaskRequest     = 17
	ICMPv4TypeAddressMaskReply       = 18
)

const (
	// DestinationUnreachable
	ICMPv4CodeNet                 = 0
	ICMPv4CodeHost                = 1
	ICMPv4CodeProtocol            = 2
	ICMPv4CodePort                = 3
	ICMPv4CodeFragmentationNeeded = 4
	ICMPv4CodeSourceRoutingFailed = 5
	ICMPv4CodeNetUnknown          = 6
	ICMPv4CodeHostUnknown         = 7
	ICMPv4CodeSourceIsolated      = 8
	ICMPv4CodeNetAdminProhibited  = 9
	ICMPv4CodeHostAdminProhibited = 10
	ICMPv4CodeNetTOS              = 11
	ICMPv4CodeHostTOS             = 12
	ICMPv4CodeCommAdminProhibited = 13
	ICMPv4CodeHostPrecedence      = 14
	ICMPv4CodePrecedenceCutoff    = 15

	// TimeExceeded
	ICMPv4CodeTTLExceeded                    = 0
	ICMPv4CodeFragmentReassemblyTimeExceeded = 1

	// ParameterProblem
	ICMPv4CodePointerIndicatesError = 0
	ICMPv4CodeMissingOption         = 1
	ICMPv4CodeBadLength             = 2

	// Redirect
	// ICMPv4CodeNet  = same as for DestinationUnreachable
	// ICMPv4CodeHost = same as for DestinationUnreachable
	ICMPv4CodeTOSNet  = 2
	ICMPv4CodeTOSHost = 3
)

type icmpv4TypeCodeInfoStruct struct {
	typeStr string
	codeStr *map[uint8]string
}

var (
	icmpv4TypeCodeInfo = map[uint8]icmpv4TypeCodeInfoStruct{
		ICMPv4TypeDestinationUnreachable: icmpv4TypeCodeInfoStruct{
			"DestinationUnreachable", &map[uint8]string{
				ICMPv4CodeNet:                 "Net",
				ICMPv4CodeHost:                "Host",
				ICMPv4CodeProtocol:            "Protocol",
				ICMPv4CodePort:                "Port",
				ICMPv4CodeFragmentationNeeded: "FragmentationNeeded",
				ICMPv4CodeSourceRoutingFailed: "SourceRoutingFailed",
				ICMPv4CodeNetUnknown:          "NetUnknown",
				ICMPv4CodeHostUnknown:         "HostUnknown",
				ICMPv4CodeSourceIsolated:      "SourceIsolated",
				ICMPv4CodeNetAdminProhibited:  "NetAdminProhibited",
				ICMPv4CodeHostAdminProhibited: "HostAdminProhibited",
				ICMPv4CodeNetTOS:              "NetTOS",
				ICMPv4CodeHostTOS:             "HostTOS",
				ICMPv4CodeCommAdminProhibited: "CommAdminProhibited",
				ICMPv4CodeHostPrecedence:      "HostPrecedence",
				ICMPv4CodePrecedenceCutoff:    "PrecedenceCutoff",
			},
		},
		ICMPv4TypeTimeExceeded: icmpv4TypeCodeInfoStruct{
			"TimeExceeded", &map[uint8]string{
				ICMPv4CodeTTLExceeded:                    "TTLExceeded",
				ICMPv4CodeFragmentReassemblyTimeExceeded: "FragmentReassemblyTimeExceeded",
			},
		},
		ICMPv4TypeParameterProblem: icmpv4TypeCodeInfoStruct{
			"ParameterProblem", &map[uint8]string{
				ICMPv4CodePointerIndicatesError: "PointerIndicatesError",
				ICMPv4CodeMissingOption:         "MissingOption",
				ICMPv4CodeBadLength:             "BadLength",
			},
		},
		ICMPv4TypeSourceQuench: icmpv4TypeCodeInfoStruct{
			"SourceQuench", nil,
		},
		ICMPv4TypeRedirect: icmpv4TypeCodeInfoStruct{
			"Redirect", &map[uint8]string{
				ICMPv4CodeNet:     "Net",
				ICMPv4CodeHost:    "Host",
				ICMPv4CodeTOSNet:  "TOS+Net",
				ICMPv4CodeTOSHost: "TOS+Host",
			},
		},
		ICMPv4TypeEchoRequest: icmpv4TypeCodeInfoStruct{
			"EchoRequest", nil,
		},
		ICMPv4TypeEchoReply: icmpv4TypeCodeInfoStruct{
			"EchoReply", nil,
		},
		ICMPv4TypeTimestampRequest: icmpv4TypeCodeInfoStruct{
			"TimestampRequest", nil,
		},
		ICMPv4TypeTimestampReply: icmpv4TypeCodeInfoStruct{
			"TimestampReply", nil,
		},
		ICMPv4TypeInfoRequest: icmpv4TypeCodeInfoStruct{
			"InfoRequest", nil,
		},
		ICMPv4TypeInfoReply: icmpv4TypeCodeInfoStruct{
			"InfoReply", nil,
		},
		ICMPv4TypeRouterSolicitation: icmpv4TypeCodeInfoStruct{
			"RouterSolicitation", nil,
		},
		ICMPv4TypeRouterAdvertisement: icmpv4TypeCodeInfoStruct{
			"RouterAdvertisement", nil,
		},
		ICMPv4TypeAddressMaskRequest: icmpv4TypeCodeInfoStruct{
			"AddressMaskRequest", nil,
		},
		ICMPv4TypeAddressMaskReply: icmpv4TypeCodeInfoStruct{
			"AddressMaskReply", nil,
		},
	}
)

type ICMPv4TypeCode uint16

// Type returns the ICMPv4 type field.
func (a ICMPv4TypeCode) Type() uint8 {
	return uint8(a >> 8)
}

// Code returns the ICMPv4 code field.
func (a ICMPv4TypeCode) Code() uint8 {
	return uint8(a)
}

func (a ICMPv4TypeCode) String() string {
	t, c := a.Type(), a.Code()
	strInfo, ok := icmpv4TypeCodeInfo[t]
	if !ok {
		// Unknown ICMPv4 type field
		return fmt.Sprintf("%d(%d)", t, c)
	}
	typeStr := strInfo.typeStr
	if strInfo.codeStr == nil && c == 0 {
		// The ICMPv4 type does not make use of the code field
		return fmt.Sprintf("%s", strInfo.typeStr)
	}
	if strInfo.codeStr == nil && c != 0 {
		// The ICMPv4 type does not make use of the code field, but it is present anyway
		return fmt.Sprintf("%s(Code: %d)", typeStr, c)
	}
	codeStr, ok := (*strInfo.codeStr)[c]
	if !ok {
		// We don't know this ICMPv4 code; print the numerical value
		return fmt.Sprintf("%s(Code: %d)", typeStr, c)
	}
	return fmt.Sprintf("%s(%s)", typeStr, codeStr)
}

func (a ICMPv4TypeCode) GoString() string {
	t := reflect.TypeOf(a)
	return fmt.Sprintf("%s(%d, %d)", t.String(), a.Type(), a.Code())
}

// SerializeTo writes the ICMPv4TypeCode value to the 'bytes' buffer.
func (a ICMPv4TypeCode) SerializeTo(bytes []byte) {
	binary.BigEndian.PutUint16(bytes, uint16(a))
}

// CreateICMPv4TypeCode is a convenience function to create an ICMPv4TypeCode
// gopacket type from the ICMPv4 type and code values.
func CreateICMPv4TypeCode(typ uint8, code uint8) ICMPv4TypeCode {
	return ICMPv4TypeCode(binary.BigEndian.Uint16([]byte{typ, code}))
}

// ICMPv4 is the layer for IPv4 ICMP packet data.
type ICMPv4 struct {
	BaseLayer
	TypeCode ICMPv4TypeCode
	Checksum uint16
	Id       uint16
	Seq      uint16
}

// LayerType returns LayerTypeICMPv4.
func (i *ICMPv4) LayerType() gopacket.LayerType { return LayerTypeICMPv4 }

// DecodeFromBytes decodes the given bytes into this layer.
func (i *ICMPv4) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 8 {
		df.SetTruncated()
		return errors.New("ICMP layer less then 8 bytes for ICMPv4 packet")
	}
	i.TypeCode = CreateICMPv4TypeCode(data[0], data[1])
	i.Checksum = binary.BigEndian.Uint16(data[2:4])
	i.Id = binary.BigEndian.Uint16(data[4:6])
	i.Seq = binary.BigEndian.Uint16(data[6:8])
	i.BaseLayer = BaseLayer{data[:8], data[8:]}
	return nil
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (i *ICMPv4) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	bytes, err := b.PrependBytes(8)
	if err != nil {
		return err
	}
	i.TypeCode.SerializeTo(bytes)
	binary.BigEndian.PutUint16(bytes[4:], i.Id)
	binary.BigEndian.PutUint16(bytes[6:], i.Seq)
	if opts.ComputeChecksums {
		bytes[2] = 0
		bytes[3] = 0
		i.Checksum = tcpipChecksum(b.Bytes(), 0)
	}
	binary.BigEndian.PutUint16(bytes[2:], i.Checksum)
	return nil
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (i *ICMPv4) CanDecode() gopacket.LayerClass {
	return LayerTypeICMPv4
}

// NextLayerType returns the layer type contained by this DecodingLayer.
func (i *ICMPv4) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

func decodeICMPv4(data []byte, p gopacket.PacketBuilder) error {
	i := &ICMPv4{}
	return decodingLayerDecoder(i, data, p)
}
