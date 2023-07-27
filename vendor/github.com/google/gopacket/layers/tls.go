// Copyright 2018 The GoPacket Authors. All rights reserved.
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

// TLSType defines the type of data after the TLS Record
type TLSType uint8

// TLSType known values.
const (
	TLSChangeCipherSpec TLSType = 20
	TLSAlert            TLSType = 21
	TLSHandshake        TLSType = 22
	TLSApplicationData  TLSType = 23
	TLSUnknown          TLSType = 255
)

// String shows the register type nicely formatted
func (tt TLSType) String() string {
	switch tt {
	default:
		return "Unknown"
	case TLSChangeCipherSpec:
		return "Change Cipher Spec"
	case TLSAlert:
		return "Alert"
	case TLSHandshake:
		return "Handshake"
	case TLSApplicationData:
		return "Application Data"
	}
}

// TLSVersion represents the TLS version in numeric format
type TLSVersion uint16

// Strings shows the TLS version nicely formatted
func (tv TLSVersion) String() string {
	switch tv {
	default:
		return "Unknown"
	case 0x0200:
		return "SSL 2.0"
	case 0x0300:
		return "SSL 3.0"
	case 0x0301:
		return "TLS 1.0"
	case 0x0302:
		return "TLS 1.1"
	case 0x0303:
		return "TLS 1.2"
	case 0x0304:
		return "TLS 1.3"
	}
}

// TLS is specified in RFC 5246
//
//  TLS Record Protocol
//  0  1  2  3  4  5  6  7  8
//  +--+--+--+--+--+--+--+--+
//  |     Content Type      |
//  +--+--+--+--+--+--+--+--+
//  |    Version (major)    |
//  +--+--+--+--+--+--+--+--+
//  |    Version (minor)    |
//  +--+--+--+--+--+--+--+--+
//  |        Length         |
//  +--+--+--+--+--+--+--+--+
//  |        Length         |
//  +--+--+--+--+--+--+--+--+

// TLS is actually a slide of TLSrecord structures
type TLS struct {
	BaseLayer

	// TLS Records
	ChangeCipherSpec []TLSChangeCipherSpecRecord
	Handshake        []TLSHandshakeRecord
	AppData          []TLSAppDataRecord
	Alert            []TLSAlertRecord
}

// TLSRecordHeader contains all the information that each TLS Record types should have
type TLSRecordHeader struct {
	ContentType TLSType
	Version     TLSVersion
	Length      uint16
}

// LayerType returns gopacket.LayerTypeTLS.
func (t *TLS) LayerType() gopacket.LayerType { return LayerTypeTLS }

// decodeTLS decodes the byte slice into a TLS type. It also
// setups the application Layer in PacketBuilder.
func decodeTLS(data []byte, p gopacket.PacketBuilder) error {
	t := &TLS{}
	err := t.DecodeFromBytes(data, p)
	if err != nil {
		return err
	}
	p.AddLayer(t)
	p.SetApplicationLayer(t)
	return nil
}

// DecodeFromBytes decodes the slice into the TLS struct.
func (t *TLS) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	t.BaseLayer.Contents = data
	t.BaseLayer.Payload = nil

	t.ChangeCipherSpec = t.ChangeCipherSpec[:0]
	t.Handshake = t.Handshake[:0]
	t.AppData = t.AppData[:0]
	t.Alert = t.Alert[:0]

	return t.decodeTLSRecords(data, df)
}

func (t *TLS) decodeTLSRecords(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 5 {
		df.SetTruncated()
		return errors.New("TLS record too short")
	}

	// since there are no further layers, the baselayer's content is
	// pointing to this layer
	// TODO: Consider removing this
	t.BaseLayer = BaseLayer{Contents: data[:len(data)]}

	var h TLSRecordHeader
	h.ContentType = TLSType(data[0])
	h.Version = TLSVersion(binary.BigEndian.Uint16(data[1:3]))
	h.Length = binary.BigEndian.Uint16(data[3:5])

	if h.ContentType.String() == "Unknown" {
		return errors.New("Unknown TLS record type")
	}

	hl := 5 // header length
	tl := hl + int(h.Length)
	if len(data) < tl {
		df.SetTruncated()
		return errors.New("TLS packet length mismatch")
	}

	switch h.ContentType {
	default:
		return errors.New("Unknown TLS record type")
	case TLSChangeCipherSpec:
		var r TLSChangeCipherSpecRecord
		e := r.decodeFromBytes(h, data[hl:tl], df)
		if e != nil {
			return e
		}
		t.ChangeCipherSpec = append(t.ChangeCipherSpec, r)
	case TLSAlert:
		var r TLSAlertRecord
		e := r.decodeFromBytes(h, data[hl:tl], df)
		if e != nil {
			return e
		}
		t.Alert = append(t.Alert, r)
	case TLSHandshake:
		var r TLSHandshakeRecord
		e := r.decodeFromBytes(h, data[hl:tl], df)
		if e != nil {
			return e
		}
		t.Handshake = append(t.Handshake, r)
	case TLSApplicationData:
		var r TLSAppDataRecord
		e := r.decodeFromBytes(h, data[hl:tl], df)
		if e != nil {
			return e
		}
		t.AppData = append(t.AppData, r)
	}

	if len(data) == tl {
		return nil
	}
	return t.decodeTLSRecords(data[tl:len(data)], df)
}

// CanDecode implements gopacket.DecodingLayer.
func (t *TLS) CanDecode() gopacket.LayerClass {
	return LayerTypeTLS
}

// NextLayerType implements gopacket.DecodingLayer.
func (t *TLS) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypeZero
}

// Payload returns nil, since TLS encrypted payload is inside TLSAppDataRecord
func (t *TLS) Payload() []byte {
	return nil
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
func (t *TLS) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	totalLength := 0
	for _, record := range t.ChangeCipherSpec {
		if opts.FixLengths {
			record.Length = 1
		}
		totalLength += 5 + 1 // length of header + record
	}
	for range t.Handshake {
		totalLength += 5
		// TODO
	}
	for _, record := range t.AppData {
		if opts.FixLengths {
			record.Length = uint16(len(record.Payload))
		}
		totalLength += 5 + len(record.Payload)
	}
	for _, record := range t.Alert {
		if len(record.EncryptedMsg) == 0 {
			if opts.FixLengths {
				record.Length = 2
			}
			totalLength += 5 + 2
		} else {
			if opts.FixLengths {
				record.Length = uint16(len(record.EncryptedMsg))
			}
			totalLength += 5 + len(record.EncryptedMsg)
		}
	}
	data, err := b.PrependBytes(totalLength)
	if err != nil {
		return err
	}
	off := 0
	for _, record := range t.ChangeCipherSpec {
		off = encodeHeader(record.TLSRecordHeader, data, off)
		data[off] = byte(record.Message)
		off++
	}
	for _, record := range t.Handshake {
		off = encodeHeader(record.TLSRecordHeader, data, off)
		// TODO
	}
	for _, record := range t.AppData {
		off = encodeHeader(record.TLSRecordHeader, data, off)
		copy(data[off:], record.Payload)
		off += len(record.Payload)
	}
	for _, record := range t.Alert {
		off = encodeHeader(record.TLSRecordHeader, data, off)
		if len(record.EncryptedMsg) == 0 {
			data[off] = byte(record.Level)
			data[off+1] = byte(record.Description)
			off += 2
		} else {
			copy(data[off:], record.EncryptedMsg)
			off += len(record.EncryptedMsg)
		}
	}
	return nil
}

func encodeHeader(header TLSRecordHeader, data []byte, offset int) int {
	data[offset] = byte(header.ContentType)
	binary.BigEndian.PutUint16(data[offset+1:], uint16(header.Version))
	binary.BigEndian.PutUint16(data[offset+3:], header.Length)

	return offset + 5
}
