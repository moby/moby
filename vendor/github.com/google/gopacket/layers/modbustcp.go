// Copyright 2018, The GoPacket Authors, All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.
//
//******************************************************************************

package layers

import (
	"encoding/binary"
	"errors"
	"github.com/google/gopacket"
)

//******************************************************************************
//
// ModbusTCP Decoding Layer
// ------------------------------------------
// This file provides a GoPacket decoding layer for ModbusTCP.
//
//******************************************************************************

const mbapRecordSizeInBytes int = 7
const modbusPDUMinimumRecordSizeInBytes int = 2
const modbusPDUMaximumRecordSizeInBytes int = 253

// ModbusProtocol type
type ModbusProtocol uint16

// ModbusProtocol known values.
const (
	ModbusProtocolModbus ModbusProtocol = 0
)

func (mp ModbusProtocol) String() string {
	switch mp {
	default:
		return "Unknown"
	case ModbusProtocolModbus:
		return "Modbus"
	}
}

//******************************************************************************

// ModbusTCP Type
// --------
// Type ModbusTCP implements the DecodingLayer interface. Each ModbusTCP object
// represents in a structured form the MODBUS Application Protocol header (MBAP) record present as the TCP
// payload in an ModbusTCP TCP packet.
//
type ModbusTCP struct {
	BaseLayer // Stores the packet bytes and payload (Modbus PDU) bytes .

	TransactionIdentifier uint16         // Identification of a MODBUS Request/Response transaction
	ProtocolIdentifier    ModbusProtocol // It is used for intra-system multiplexing
	Length                uint16         // Number of following bytes (includes 1 byte for UnitIdentifier + Modbus data length
	UnitIdentifier        uint8          // Identification of a remote slave connected on a serial line or on other buses
}

//******************************************************************************

// LayerType returns the layer type of the ModbusTCP object, which is LayerTypeModbusTCP.
func (d *ModbusTCP) LayerType() gopacket.LayerType {
	return LayerTypeModbusTCP
}

//******************************************************************************

// decodeModbusTCP analyses a byte slice and attempts to decode it as an ModbusTCP
// record of a TCP packet.
//
// If it succeeds, it loads p with information about the packet and returns nil.
// If it fails, it returns an error (non nil).
//
// This function is employed in layertypes.go to register the ModbusTCP layer.
func decodeModbusTCP(data []byte, p gopacket.PacketBuilder) error {

	// Attempt to decode the byte slice.
	d := &ModbusTCP{}
	err := d.DecodeFromBytes(data, p)
	if err != nil {
		return err
	}
	// If the decoding worked, add the layer to the packet and set it
	// as the application layer too, if there isn't already one.
	p.AddLayer(d)
	p.SetApplicationLayer(d)

	return p.NextDecoder(d.NextLayerType())

}

//******************************************************************************

// DecodeFromBytes analyses a byte slice and attempts to decode it as an ModbusTCP
// record of a TCP packet.
//
// Upon succeeds, it loads the ModbusTCP object with information about the packet
// and returns nil.
// Upon failure, it returns an error (non nil).
func (d *ModbusTCP) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {

	// If the data block is too short to be a MBAP record, then return an error.
	if len(data) < mbapRecordSizeInBytes+modbusPDUMinimumRecordSizeInBytes {
		df.SetTruncated()
		return errors.New("ModbusTCP packet too short")
	}

	if len(data) > mbapRecordSizeInBytes+modbusPDUMaximumRecordSizeInBytes {
		df.SetTruncated()
		return errors.New("ModbusTCP packet too long")
	}

	// ModbusTCP type embeds type BaseLayer which contains two fields:
	//    Contents is supposed to contain the bytes of the data at this level (MPBA).
	//    Payload is supposed to contain the payload of this level (PDU).
	d.BaseLayer = BaseLayer{Contents: data[:mbapRecordSizeInBytes], Payload: data[mbapRecordSizeInBytes:len(data)]}

	// Extract the fields from the block of bytes.
	// The fields can just be copied in big endian order.
	d.TransactionIdentifier = binary.BigEndian.Uint16(data[:2])
	d.ProtocolIdentifier = ModbusProtocol(binary.BigEndian.Uint16(data[2:4]))
	d.Length = binary.BigEndian.Uint16(data[4:6])

	// Length should have the size of the payload plus one byte (size of UnitIdentifier)
	if d.Length != uint16(len(d.BaseLayer.Payload)+1) {
		df.SetTruncated()
		return errors.New("ModbusTCP packet with wrong field value (Length)")
	}
	d.UnitIdentifier = uint8(data[6])

	return nil
}

//******************************************************************************

// NextLayerType returns the layer type of the ModbusTCP payload, which is LayerTypePayload.
func (d *ModbusTCP) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

//******************************************************************************

// Payload returns Modbus Protocol Data Unit (PDU) composed by Function Code and Data, it is carried within ModbusTCP packets
func (d *ModbusTCP) Payload() []byte {
	return d.BaseLayer.Payload
}
