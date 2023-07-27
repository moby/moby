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

// EAPOL defines an EAP over LAN (802.1x) layer.
type EAPOL struct {
	BaseLayer
	Version uint8
	Type    EAPOLType
	Length  uint16
}

// LayerType returns LayerTypeEAPOL.
func (e *EAPOL) LayerType() gopacket.LayerType { return LayerTypeEAPOL }

// DecodeFromBytes decodes the given bytes into this layer.
func (e *EAPOL) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 4 {
		df.SetTruncated()
		return fmt.Errorf("EAPOL length %d too short", len(data))
	}
	e.Version = data[0]
	e.Type = EAPOLType(data[1])
	e.Length = binary.BigEndian.Uint16(data[2:4])
	e.BaseLayer = BaseLayer{data[:4], data[4:]}
	return nil
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer
func (e *EAPOL) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	bytes, _ := b.PrependBytes(4)
	bytes[0] = e.Version
	bytes[1] = byte(e.Type)
	binary.BigEndian.PutUint16(bytes[2:], e.Length)
	return nil
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (e *EAPOL) CanDecode() gopacket.LayerClass {
	return LayerTypeEAPOL
}

// NextLayerType returns the layer type contained by this DecodingLayer.
func (e *EAPOL) NextLayerType() gopacket.LayerType {
	return e.Type.LayerType()
}

func decodeEAPOL(data []byte, p gopacket.PacketBuilder) error {
	e := &EAPOL{}
	return decodingLayerDecoder(e, data, p)
}

// EAPOLKeyDescriptorType is an enumeration of key descriptor types
// as specified by 802.1x in the EAPOL-Key frame
type EAPOLKeyDescriptorType uint8

// Enumeration of EAPOLKeyDescriptorType
const (
	EAPOLKeyDescriptorTypeRC4   EAPOLKeyDescriptorType = 1
	EAPOLKeyDescriptorTypeDot11 EAPOLKeyDescriptorType = 2
	EAPOLKeyDescriptorTypeWPA   EAPOLKeyDescriptorType = 254
)

func (kdt EAPOLKeyDescriptorType) String() string {
	switch kdt {
	case EAPOLKeyDescriptorTypeRC4:
		return "RC4"
	case EAPOLKeyDescriptorTypeDot11:
		return "802.11"
	case EAPOLKeyDescriptorTypeWPA:
		return "WPA"
	default:
		return fmt.Sprintf("unknown descriptor type %d", kdt)
	}
}

// EAPOLKeyDescriptorVersion is an enumeration of versions specifying the
// encryption algorithm for the key data and the authentication for the
// message integrity code (MIC)
type EAPOLKeyDescriptorVersion uint8

// Enumeration of EAPOLKeyDescriptorVersion
const (
	EAPOLKeyDescriptorVersionOther       EAPOLKeyDescriptorVersion = 0
	EAPOLKeyDescriptorVersionRC4HMACMD5  EAPOLKeyDescriptorVersion = 1
	EAPOLKeyDescriptorVersionAESHMACSHA1 EAPOLKeyDescriptorVersion = 2
	EAPOLKeyDescriptorVersionAES128CMAC  EAPOLKeyDescriptorVersion = 3
)

func (v EAPOLKeyDescriptorVersion) String() string {
	switch v {
	case EAPOLKeyDescriptorVersionOther:
		return "Other"
	case EAPOLKeyDescriptorVersionRC4HMACMD5:
		return "RC4-HMAC-MD5"
	case EAPOLKeyDescriptorVersionAESHMACSHA1:
		return "AES-HMAC-SHA1-128"
	case EAPOLKeyDescriptorVersionAES128CMAC:
		return "AES-128-CMAC"
	default:
		return fmt.Sprintf("unknown version %d", v)
	}
}

// EAPOLKeyType is an enumeration of key derivation types describing
// the purpose of the keys being derived.
type EAPOLKeyType uint8

// Enumeration of EAPOLKeyType
const (
	EAPOLKeyTypeGroupSMK EAPOLKeyType = 0
	EAPOLKeyTypePairwise EAPOLKeyType = 1
)

func (kt EAPOLKeyType) String() string {
	switch kt {
	case EAPOLKeyTypeGroupSMK:
		return "Group/SMK"
	case EAPOLKeyTypePairwise:
		return "Pairwise"
	default:
		return fmt.Sprintf("unknown key type %d", kt)
	}
}

// EAPOLKey defines an EAPOL-Key frame for 802.1x authentication
type EAPOLKey struct {
	BaseLayer
	KeyDescriptorType    EAPOLKeyDescriptorType
	KeyDescriptorVersion EAPOLKeyDescriptorVersion
	KeyType              EAPOLKeyType
	KeyIndex             uint8
	Install              bool
	KeyACK               bool
	KeyMIC               bool
	Secure               bool
	MICError             bool
	Request              bool
	HasEncryptedKeyData  bool
	SMKMessage           bool
	KeyLength            uint16
	ReplayCounter        uint64
	Nonce                []byte
	IV                   []byte
	RSC                  uint64
	ID                   uint64
	MIC                  []byte
	KeyDataLength        uint16
	EncryptedKeyData     []byte
}

// LayerType returns LayerTypeEAPOLKey.
func (ek *EAPOLKey) LayerType() gopacket.LayerType {
	return LayerTypeEAPOLKey
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (ek *EAPOLKey) CanDecode() gopacket.LayerType {
	return LayerTypeEAPOLKey
}

// NextLayerType returns layers.LayerTypeDot11InformationElement if the key
// data exists and is unencrypted, otherwise it does not expect a next layer.
func (ek *EAPOLKey) NextLayerType() gopacket.LayerType {
	if !ek.HasEncryptedKeyData && ek.KeyDataLength > 0 {
		return LayerTypeDot11InformationElement
	}
	return gopacket.LayerTypePayload
}

const eapolKeyFrameLen = 95

// DecodeFromBytes decodes the given bytes into this layer.
func (ek *EAPOLKey) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < eapolKeyFrameLen {
		df.SetTruncated()
		return fmt.Errorf("EAPOLKey length %v too short, %v required",
			len(data), eapolKeyFrameLen)
	}

	ek.KeyDescriptorType = EAPOLKeyDescriptorType(data[0])

	info := binary.BigEndian.Uint16(data[1:3])
	ek.KeyDescriptorVersion = EAPOLKeyDescriptorVersion(info & 0x0007)
	ek.KeyType = EAPOLKeyType((info & 0x0008) >> 3)
	ek.KeyIndex = uint8((info & 0x0030) >> 4)
	ek.Install = (info & 0x0040) != 0
	ek.KeyACK = (info & 0x0080) != 0
	ek.KeyMIC = (info & 0x0100) != 0
	ek.Secure = (info & 0x0200) != 0
	ek.MICError = (info & 0x0400) != 0
	ek.Request = (info & 0x0800) != 0
	ek.HasEncryptedKeyData = (info & 0x1000) != 0
	ek.SMKMessage = (info & 0x2000) != 0

	ek.KeyLength = binary.BigEndian.Uint16(data[3:5])
	ek.ReplayCounter = binary.BigEndian.Uint64(data[5:13])

	ek.Nonce = data[13:45]
	ek.IV = data[45:61]
	ek.RSC = binary.BigEndian.Uint64(data[61:69])
	ek.ID = binary.BigEndian.Uint64(data[69:77])
	ek.MIC = data[77:93]

	ek.KeyDataLength = binary.BigEndian.Uint16(data[93:95])

	totalLength := eapolKeyFrameLen + int(ek.KeyDataLength)
	if len(data) < totalLength {
		df.SetTruncated()
		return fmt.Errorf("EAPOLKey data length %d too short, %d required",
			len(data)-eapolKeyFrameLen, ek.KeyDataLength)
	}

	if ek.HasEncryptedKeyData {
		ek.EncryptedKeyData = data[eapolKeyFrameLen:totalLength]
		ek.BaseLayer = BaseLayer{
			Contents: data[:totalLength],
			Payload:  data[totalLength:],
		}
	} else {
		ek.BaseLayer = BaseLayer{
			Contents: data[:eapolKeyFrameLen],
			Payload:  data[eapolKeyFrameLen:],
		}
	}

	return nil
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (ek *EAPOLKey) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	buf, err := b.PrependBytes(eapolKeyFrameLen + len(ek.EncryptedKeyData))
	if err != nil {
		return err
	}

	buf[0] = byte(ek.KeyDescriptorType)

	var info uint16
	info |= uint16(ek.KeyDescriptorVersion)
	info |= uint16(ek.KeyType) << 3
	info |= uint16(ek.KeyIndex) << 4
	if ek.Install {
		info |= 0x0040
	}
	if ek.KeyACK {
		info |= 0x0080
	}
	if ek.KeyMIC {
		info |= 0x0100
	}
	if ek.Secure {
		info |= 0x0200
	}
	if ek.MICError {
		info |= 0x0400
	}
	if ek.Request {
		info |= 0x0800
	}
	if ek.HasEncryptedKeyData {
		info |= 0x1000
	}
	if ek.SMKMessage {
		info |= 0x2000
	}
	binary.BigEndian.PutUint16(buf[1:3], info)

	binary.BigEndian.PutUint16(buf[3:5], ek.KeyLength)
	binary.BigEndian.PutUint64(buf[5:13], ek.ReplayCounter)

	copy(buf[13:45], ek.Nonce)
	copy(buf[45:61], ek.IV)
	binary.BigEndian.PutUint64(buf[61:69], ek.RSC)
	binary.BigEndian.PutUint64(buf[69:77], ek.ID)
	copy(buf[77:93], ek.MIC)

	binary.BigEndian.PutUint16(buf[93:95], ek.KeyDataLength)
	if len(ek.EncryptedKeyData) > 0 {
		copy(buf[95:95+len(ek.EncryptedKeyData)], ek.EncryptedKeyData)
	}

	return nil
}

func decodeEAPOLKey(data []byte, p gopacket.PacketBuilder) error {
	ek := &EAPOLKey{}
	return decodingLayerDecoder(ek, data, p)
}
