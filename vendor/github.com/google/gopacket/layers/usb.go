// Copyright 2014 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"encoding/binary"
	"errors"
	"github.com/google/gopacket"
)

type USBEventType uint8

const (
	USBEventTypeSubmit   USBEventType = 'S'
	USBEventTypeComplete USBEventType = 'C'
	USBEventTypeError    USBEventType = 'E'
)

func (a USBEventType) String() string {
	switch a {
	case USBEventTypeSubmit:
		return "SUBMIT"
	case USBEventTypeComplete:
		return "COMPLETE"
	case USBEventTypeError:
		return "ERROR"
	default:
		return "Unknown event type"
	}
}

type USBRequestBlockSetupRequest uint8

const (
	USBRequestBlockSetupRequestGetStatus        USBRequestBlockSetupRequest = 0x00
	USBRequestBlockSetupRequestClearFeature     USBRequestBlockSetupRequest = 0x01
	USBRequestBlockSetupRequestSetFeature       USBRequestBlockSetupRequest = 0x03
	USBRequestBlockSetupRequestSetAddress       USBRequestBlockSetupRequest = 0x05
	USBRequestBlockSetupRequestGetDescriptor    USBRequestBlockSetupRequest = 0x06
	USBRequestBlockSetupRequestSetDescriptor    USBRequestBlockSetupRequest = 0x07
	USBRequestBlockSetupRequestGetConfiguration USBRequestBlockSetupRequest = 0x08
	USBRequestBlockSetupRequestSetConfiguration USBRequestBlockSetupRequest = 0x09
	USBRequestBlockSetupRequestSetIdle          USBRequestBlockSetupRequest = 0x0a
)

func (a USBRequestBlockSetupRequest) String() string {
	switch a {
	case USBRequestBlockSetupRequestGetStatus:
		return "GET_STATUS"
	case USBRequestBlockSetupRequestClearFeature:
		return "CLEAR_FEATURE"
	case USBRequestBlockSetupRequestSetFeature:
		return "SET_FEATURE"
	case USBRequestBlockSetupRequestSetAddress:
		return "SET_ADDRESS"
	case USBRequestBlockSetupRequestGetDescriptor:
		return "GET_DESCRIPTOR"
	case USBRequestBlockSetupRequestSetDescriptor:
		return "SET_DESCRIPTOR"
	case USBRequestBlockSetupRequestGetConfiguration:
		return "GET_CONFIGURATION"
	case USBRequestBlockSetupRequestSetConfiguration:
		return "SET_CONFIGURATION"
	case USBRequestBlockSetupRequestSetIdle:
		return "SET_IDLE"
	default:
		return "UNKNOWN"
	}
}

type USBTransportType uint8

const (
	USBTransportTypeTransferIn  USBTransportType = 0x80 // Indicates send or receive
	USBTransportTypeIsochronous USBTransportType = 0x00 // Isochronous transfers occur continuously and periodically. They typically contain time sensitive information, such as an audio or video stream.
	USBTransportTypeInterrupt   USBTransportType = 0x01 // Interrupt transfers are typically non-periodic, small device "initiated" communication requiring bounded latency, such as pointing devices or keyboards.
	USBTransportTypeControl     USBTransportType = 0x02 // Control transfers are typically used for command and status operations.
	USBTransportTypeBulk        USBTransportType = 0x03 // Bulk transfers can be used for large bursty data, using all remaining available bandwidth, no guarantees on bandwidth or latency, such as file transfers.
)

type USBDirectionType uint8

const (
	USBDirectionTypeUnknown USBDirectionType = iota
	USBDirectionTypeIn
	USBDirectionTypeOut
)

func (a USBDirectionType) String() string {
	switch a {
	case USBDirectionTypeIn:
		return "In"
	case USBDirectionTypeOut:
		return "Out"
	default:
		return "Unknown direction type"
	}
}

// The reference at http://www.beyondlogic.org/usbnutshell/usb1.shtml contains more information about the protocol.
type USB struct {
	BaseLayer
	ID             uint64
	EventType      USBEventType
	TransferType   USBTransportType
	Direction      USBDirectionType
	EndpointNumber uint8
	DeviceAddress  uint8
	BusID          uint16
	TimestampSec   int64
	TimestampUsec  int32
	Setup          bool
	Data           bool
	Status         int32
	UrbLength      uint32
	UrbDataLength  uint32

	UrbInterval            uint32
	UrbStartFrame          uint32
	UrbCopyOfTransferFlags uint32
	IsoNumDesc             uint32
}

func (u *USB) LayerType() gopacket.LayerType { return LayerTypeUSB }

func (m *USB) NextLayerType() gopacket.LayerType {
	if m.Setup {
		return LayerTypeUSBRequestBlockSetup
	} else if m.Data {
	}

	return m.TransferType.LayerType()
}

func decodeUSB(data []byte, p gopacket.PacketBuilder) error {
	d := &USB{}

	return decodingLayerDecoder(d, data, p)
}

func (m *USB) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 40 {
		df.SetTruncated()
		return errors.New("USB < 40 bytes")
	}
	m.ID = binary.LittleEndian.Uint64(data[0:8])
	m.EventType = USBEventType(data[8])
	m.TransferType = USBTransportType(data[9])

	m.EndpointNumber = data[10] & 0x7f
	if data[10]&uint8(USBTransportTypeTransferIn) > 0 {
		m.Direction = USBDirectionTypeIn
	} else {
		m.Direction = USBDirectionTypeOut
	}

	m.DeviceAddress = data[11]
	m.BusID = binary.LittleEndian.Uint16(data[12:14])

	if uint(data[14]) == 0 {
		m.Setup = true
	}

	if uint(data[15]) == 0 {
		m.Data = true
	}

	m.TimestampSec = int64(binary.LittleEndian.Uint64(data[16:24]))
	m.TimestampUsec = int32(binary.LittleEndian.Uint32(data[24:28]))
	m.Status = int32(binary.LittleEndian.Uint32(data[28:32]))
	m.UrbLength = binary.LittleEndian.Uint32(data[32:36])
	m.UrbDataLength = binary.LittleEndian.Uint32(data[36:40])

	m.Contents = data[:40]
	m.Payload = data[40:]

	if m.Setup {
		m.Payload = data[40:]
	} else if m.Data {
		m.Payload = data[uint32(len(data))-m.UrbDataLength:]
	}

	// if 64 bit, dissect_linux_usb_pseudo_header_ext
	if false {
		m.UrbInterval = binary.LittleEndian.Uint32(data[40:44])
		m.UrbStartFrame = binary.LittleEndian.Uint32(data[44:48])
		m.UrbDataLength = binary.LittleEndian.Uint32(data[48:52])
		m.IsoNumDesc = binary.LittleEndian.Uint32(data[52:56])
		m.Contents = data[:56]
		m.Payload = data[56:]
	}

	// crc5 or crc16
	// eop (end of packet)

	return nil
}

type USBRequestBlockSetup struct {
	BaseLayer
	RequestType uint8
	Request     USBRequestBlockSetupRequest
	Value       uint16
	Index       uint16
	Length      uint16
}

func (u *USBRequestBlockSetup) LayerType() gopacket.LayerType { return LayerTypeUSBRequestBlockSetup }

func (m *USBRequestBlockSetup) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

func (m *USBRequestBlockSetup) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	m.RequestType = data[0]
	m.Request = USBRequestBlockSetupRequest(data[1])
	m.Value = binary.LittleEndian.Uint16(data[2:4])
	m.Index = binary.LittleEndian.Uint16(data[4:6])
	m.Length = binary.LittleEndian.Uint16(data[6:8])
	m.Contents = data[:8]
	m.Payload = data[8:]
	return nil
}

func decodeUSBRequestBlockSetup(data []byte, p gopacket.PacketBuilder) error {
	d := &USBRequestBlockSetup{}
	return decodingLayerDecoder(d, data, p)
}

type USBControl struct {
	BaseLayer
}

func (u *USBControl) LayerType() gopacket.LayerType { return LayerTypeUSBControl }

func (m *USBControl) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

func (m *USBControl) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	m.Contents = data
	return nil
}

func decodeUSBControl(data []byte, p gopacket.PacketBuilder) error {
	d := &USBControl{}
	return decodingLayerDecoder(d, data, p)
}

type USBInterrupt struct {
	BaseLayer
}

func (u *USBInterrupt) LayerType() gopacket.LayerType { return LayerTypeUSBInterrupt }

func (m *USBInterrupt) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

func (m *USBInterrupt) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	m.Contents = data
	return nil
}

func decodeUSBInterrupt(data []byte, p gopacket.PacketBuilder) error {
	d := &USBInterrupt{}
	return decodingLayerDecoder(d, data, p)
}

type USBBulk struct {
	BaseLayer
}

func (u *USBBulk) LayerType() gopacket.LayerType { return LayerTypeUSBBulk }

func (m *USBBulk) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

func (m *USBBulk) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	m.Contents = data
	return nil
}

func decodeUSBBulk(data []byte, p gopacket.PacketBuilder) error {
	d := &USBBulk{}
	return decodingLayerDecoder(d, data, p)
}
