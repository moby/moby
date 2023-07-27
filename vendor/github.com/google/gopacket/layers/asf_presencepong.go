// Copyright 2019 The GoPacket Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file in the root of the source tree.

package layers

// This file implements the RMCP ASF Presence Pong message, specified in section
// 3.2.4.3 of
// https://www.dmtf.org/sites/default/files/standards/documents/DSP0136.pdf. It
// also contains non-competing elements from IPMI v2.0, specified in section
// 13.2.4 of
// https://www.intel.com/content/dam/www/public/us/en/documents/specification-updates/ipmi-intelligent-platform-mgt-interface-spec-2nd-gen-v2-0-spec-update.pdf.

import (
	"encoding/binary"
	"fmt"

	"github.com/google/gopacket"
)

type (
	// ASFEntity is the type of individual entities that a Presence Pong
	// response can indicate support of. The entities currently implemented by
	// the spec are IPMI and ASFv1.
	ASFEntity uint8

	// ASFInteraction is the type of individual interactions that a Presence
	// Pong response can indicate support for. The interactions currently
	// implemented by the spec are RMCP security extensions. Although not
	// specified, IPMI uses this field to indicate support for DASH, which is
	// supported as well.
	ASFInteraction uint8
)

const (
	// ASFDCMIEnterprise is the IANA-assigned Enterprise Number of the Data
	// Center Manageability Interface Forum. The Presence Pong response's
	// Enterprise field being set to this value indicates support for DCMI. The
	// DCMI spec regards the OEM field as reserved, so these should be null.
	ASFDCMIEnterprise uint32 = 36465

	// ASFPresencePongEntityIPMI ANDs with Presence Pong's supported entities
	// field if the managed system supports IPMI.
	ASFPresencePongEntityIPMI ASFEntity = 1 << 7

	// ASFPresencePongEntityASFv1 ANDs with Presence Pong's supported entities
	// field if the managed system supports ASF v1.0.
	ASFPresencePongEntityASFv1 ASFEntity = 1

	// ASFPresencePongInteractionSecurityExtensions ANDs with Presence Pong's
	// supported interactions field if the managed system supports RMCP v2.0
	// security extensions. See section 3.2.3.
	ASFPresencePongInteractionSecurityExtensions ASFInteraction = 1 << 7

	// ASFPresencePongInteractionDASH ANDs with Presence Pong's supported
	// interactions field if the managed system supports DMTF DASH. See
	// https://www.dmtf.org/standards/dash.
	ASFPresencePongInteractionDASH ASFInteraction = 1 << 5
)

// ASFPresencePong defines the structure of a Presence Pong message's payload.
// See section 3.2.4.3.
type ASFPresencePong struct {
	BaseLayer

	// Enterprise is the IANA Enterprise Number of an entity that has defined
	// OEM-specific capabilities for the managed client. If no such capabilities
	// exist, this is set to ASF's IANA Enterprise Number.
	Enterprise uint32

	// OEM identifies OEM-specific capabilities. Its structure is defined by the
	// OEM. This is set to 0s if no OEM-specific capabilities exist. This
	// implementation does not change byte order from the wire for this field.
	OEM [4]byte

	// We break out entities and interactions into separate booleans as
	// discovery is the entire point of this type of message, so we assume they
	// are accessed. It also makes gopacket's default layer printing more
	// useful.

	// IPMI is true if IPMI is supported by the managed system. There is no
	// explicit version in the specification, however given the dates, this is
	// assumed to be IPMI v1.0.  Support for IPMI is contained in the "supported
	// entities" field of the presence pong payload.
	IPMI bool

	// ASFv1 indicates support for ASF v1.0. This seems somewhat redundant as
	// ASF must be supported in order to receive a response. This is contained
	// in the "supported entities" field of the presence pong payload.
	ASFv1 bool

	// SecurityExtensions indicates support for RMCP Security Extensions,
	// specified in ASF v2.0. This will always be false for v1.x
	// implementations. This is contained in the "supported interactions" field
	// of the presence pong payload. This field is defined in ASF v1.0, but has
	// no useful value.
	SecurityExtensions bool

	// DASH is true if DMTF DASH is supported. This is not specified in ASF
	// v2.0, but in IPMI v2.0, however the former does not preclude it, so we
	// support it.
	DASH bool

	// 6 bytes reserved after the entities and interactions fields, set to 0s.
}

// SupportsDCMI returns whether the Presence Pong message indicates support for
// the Data Center Management Interface, which is an extension of IPMI v2.0.
func (a *ASFPresencePong) SupportsDCMI() bool {
	return a.Enterprise == ASFDCMIEnterprise && a.IPMI && a.ASFv1
}

// LayerType returns LayerTypeASFPresencePong. It partially satisfies Layer and
// SerializableLayer.
func (*ASFPresencePong) LayerType() gopacket.LayerType {
	return LayerTypeASFPresencePong
}

// CanDecode returns LayerTypeASFPresencePong. It partially satisfies
// DecodingLayer.
func (a *ASFPresencePong) CanDecode() gopacket.LayerClass {
	return a.LayerType()
}

// DecodeFromBytes makes the layer represent the provided bytes. It partially
// satisfies DecodingLayer.
func (a *ASFPresencePong) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 16 {
		df.SetTruncated()
		return fmt.Errorf("invalid ASF presence pong payload, length %v less than 16",
			len(data))
	}

	a.BaseLayer.Contents = data[:16]
	a.BaseLayer.Payload = data[16:]

	a.Enterprise = binary.BigEndian.Uint32(data[:4])
	copy(a.OEM[:], data[4:8]) // N.B. no byte order change
	a.IPMI = data[8]&uint8(ASFPresencePongEntityIPMI) != 0
	a.ASFv1 = data[8]&uint8(ASFPresencePongEntityASFv1) != 0
	a.SecurityExtensions = data[9]&uint8(ASFPresencePongInteractionSecurityExtensions) != 0
	a.DASH = data[9]&uint8(ASFPresencePongInteractionDASH) != 0
	// ignore remaining 6 bytes; should be set to 0s
	return nil
}

// NextLayerType returns LayerTypePayload, as there are no further layers to
// decode. This partially satisfies DecodingLayer.
func (a *ASFPresencePong) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

// SerializeTo writes the serialized fom of this layer into the SerializeBuffer,
// partially satisfying SerializableLayer.
func (a *ASFPresencePong) SerializeTo(b gopacket.SerializeBuffer, _ gopacket.SerializeOptions) error {
	bytes, err := b.PrependBytes(16)
	if err != nil {
		return err
	}

	binary.BigEndian.PutUint32(bytes[:4], a.Enterprise)

	copy(bytes[4:8], a.OEM[:])

	bytes[8] = 0
	if a.IPMI {
		bytes[8] |= uint8(ASFPresencePongEntityIPMI)
	}
	if a.ASFv1 {
		bytes[8] |= uint8(ASFPresencePongEntityASFv1)
	}

	bytes[9] = 0
	if a.SecurityExtensions {
		bytes[9] |= uint8(ASFPresencePongInteractionSecurityExtensions)
	}
	if a.DASH {
		bytes[9] |= uint8(ASFPresencePongInteractionDASH)
	}

	// zero-out remaining 6 bytes
	for i := 10; i < len(bytes); i++ {
		bytes[i] = 0x00
	}

	return nil
}

// decodeASFPresencePong decodes the byte slice into an RMCP-ASF Presence Pong
// struct.
func decodeASFPresencePong(data []byte, p gopacket.PacketBuilder) error {
	return decodingLayerDecoder(&ASFPresencePong{}, data, p)
}
