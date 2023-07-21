// Copyright 2015 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

// http://www.tcpdump.org/linktypes/LINKTYPE_IEEE802_11_PRISM.html

package layers

import (
	"encoding/binary"
	"errors"

	"github.com/google/gopacket"
)

func decodePrismValue(data []byte, pv *PrismValue) {
	pv.DID = PrismDID(binary.LittleEndian.Uint32(data[0:4]))
	pv.Status = binary.LittleEndian.Uint16(data[4:6])
	pv.Length = binary.LittleEndian.Uint16(data[6:8])
	pv.Data = data[8 : 8+pv.Length]
}

type PrismDID uint32

const (
	PrismDIDType1HostTime                  PrismDID = 0x10044
	PrismDIDType2HostTime                  PrismDID = 0x01041
	PrismDIDType1MACTime                   PrismDID = 0x20044
	PrismDIDType2MACTime                   PrismDID = 0x02041
	PrismDIDType1Channel                   PrismDID = 0x30044
	PrismDIDType2Channel                   PrismDID = 0x03041
	PrismDIDType1RSSI                      PrismDID = 0x40044
	PrismDIDType2RSSI                      PrismDID = 0x04041
	PrismDIDType1SignalQuality             PrismDID = 0x50044
	PrismDIDType2SignalQuality             PrismDID = 0x05041
	PrismDIDType1Signal                    PrismDID = 0x60044
	PrismDIDType2Signal                    PrismDID = 0x06041
	PrismDIDType1Noise                     PrismDID = 0x70044
	PrismDIDType2Noise                     PrismDID = 0x07041
	PrismDIDType1Rate                      PrismDID = 0x80044
	PrismDIDType2Rate                      PrismDID = 0x08041
	PrismDIDType1TransmittedFrameIndicator PrismDID = 0x90044
	PrismDIDType2TransmittedFrameIndicator PrismDID = 0x09041
	PrismDIDType1FrameLength               PrismDID = 0xA0044
	PrismDIDType2FrameLength               PrismDID = 0x0A041
)

const (
	PrismType1MessageCode uint16 = 0x00000044
	PrismType2MessageCode uint16 = 0x00000041
)

func (p PrismDID) String() string {
	dids := map[PrismDID]string{
		PrismDIDType1HostTime:                  "Host Time",
		PrismDIDType2HostTime:                  "Host Time",
		PrismDIDType1MACTime:                   "MAC Time",
		PrismDIDType2MACTime:                   "MAC Time",
		PrismDIDType1Channel:                   "Channel",
		PrismDIDType2Channel:                   "Channel",
		PrismDIDType1RSSI:                      "RSSI",
		PrismDIDType2RSSI:                      "RSSI",
		PrismDIDType1SignalQuality:             "Signal Quality",
		PrismDIDType2SignalQuality:             "Signal Quality",
		PrismDIDType1Signal:                    "Signal",
		PrismDIDType2Signal:                    "Signal",
		PrismDIDType1Noise:                     "Noise",
		PrismDIDType2Noise:                     "Noise",
		PrismDIDType1Rate:                      "Rate",
		PrismDIDType2Rate:                      "Rate",
		PrismDIDType1TransmittedFrameIndicator: "Transmitted Frame Indicator",
		PrismDIDType2TransmittedFrameIndicator: "Transmitted Frame Indicator",
		PrismDIDType1FrameLength:               "Frame Length",
		PrismDIDType2FrameLength:               "Frame Length",
	}

	if str, ok := dids[p]; ok {
		return str
	}

	return "Unknown DID"
}

type PrismValue struct {
	DID    PrismDID
	Status uint16
	Length uint16
	Data   []byte
}

func (pv *PrismValue) IsSupplied() bool {
	return pv.Status == 1
}

var ErrPrismExpectedMoreData = errors.New("Expected more data.")
var ErrPrismInvalidCode = errors.New("Invalid header code.")

func decodePrismHeader(data []byte, p gopacket.PacketBuilder) error {
	d := &PrismHeader{}
	return decodingLayerDecoder(d, data, p)
}

type PrismHeader struct {
	BaseLayer
	Code       uint16
	Length     uint16
	DeviceName string
	Values     []PrismValue
}

func (m *PrismHeader) LayerType() gopacket.LayerType { return LayerTypePrismHeader }

func (m *PrismHeader) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	m.Code = binary.LittleEndian.Uint16(data[0:4])
	m.Length = binary.LittleEndian.Uint16(data[4:8])
	m.DeviceName = string(data[8:24])
	m.BaseLayer = BaseLayer{Contents: data[:m.Length], Payload: data[m.Length:len(data)]}

	switch m.Code {
	case PrismType1MessageCode:
		fallthrough
	case PrismType2MessageCode:
		// valid message code
	default:
		return ErrPrismInvalidCode
	}

	offset := uint16(24)

	m.Values = make([]PrismValue, (m.Length-offset)/12)
	for i := 0; i < len(m.Values); i++ {
		decodePrismValue(data[offset:offset+12], &m.Values[i])
		offset += 12
	}

	if offset != m.Length {
		return ErrPrismExpectedMoreData
	}

	return nil
}

func (m *PrismHeader) CanDecode() gopacket.LayerClass    { return LayerTypePrismHeader }
func (m *PrismHeader) NextLayerType() gopacket.LayerType { return LayerTypeDot11 }
