// Copyright 2016 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"encoding/binary"
	"errors"
	"net"

	"github.com/google/gopacket"
)

/*
	This layer provides decoding for Virtual Router Redundancy Protocol (VRRP) v2.
	https://tools.ietf.org/html/rfc3768#section-5
    0                   1                   2                   3
    0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |Version| Type  | Virtual Rtr ID|   Priority    | Count IP Addrs|
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |   Auth Type   |   Adver Int   |          Checksum             |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |                         IP Address (1)                        |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |                            .                                  |
   |                            .                                  |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |                         IP Address (n)                        |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |                     Authentication Data (1)                   |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |                     Authentication Data (2)                   |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
*/

type VRRPv2Type uint8
type VRRPv2AuthType uint8

const (
	VRRPv2Advertisement VRRPv2Type = 0x01 // router advertisement
)

// String conversions for VRRP message types
func (v VRRPv2Type) String() string {
	switch v {
	case VRRPv2Advertisement:
		return "VRRPv2 Advertisement"
	default:
		return ""
	}
}

const (
	VRRPv2AuthNoAuth    VRRPv2AuthType = 0x00 // No Authentication
	VRRPv2AuthReserved1 VRRPv2AuthType = 0x01 // Reserved field 1
	VRRPv2AuthReserved2 VRRPv2AuthType = 0x02 // Reserved field 2
)

func (v VRRPv2AuthType) String() string {
	switch v {
	case VRRPv2AuthNoAuth:
		return "No Authentication"
	case VRRPv2AuthReserved1:
		return "Reserved"
	case VRRPv2AuthReserved2:
		return "Reserved"
	default:
		return ""
	}
}

// VRRPv2 represents an VRRP v2 message.
type VRRPv2 struct {
	BaseLayer
	Version      uint8          // The version field specifies the VRRP protocol version of this packet (v2)
	Type         VRRPv2Type     // The type field specifies the type of this VRRP packet.  The only type defined in v2 is ADVERTISEMENT
	VirtualRtrID uint8          // identifies the virtual router this packet is reporting status for
	Priority     uint8          // specifies the sending VRRP router's priority for the virtual router (100 = default)
	CountIPAddr  uint8          // The number of IP addresses contained in this VRRP advertisement.
	AuthType     VRRPv2AuthType // identifies the authentication method being utilized
	AdverInt     uint8          // The Advertisement interval indicates the time interval (in seconds) between ADVERTISEMENTS.  The default is 1 second
	Checksum     uint16         // used to detect data corruption in the VRRP message.
	IPAddress    []net.IP       // one or more IP addresses associated with the virtual router. Specified in the CountIPAddr field.
}

// LayerType returns LayerTypeVRRP for VRRP v2 message.
func (v *VRRPv2) LayerType() gopacket.LayerType { return LayerTypeVRRP }

func (v *VRRPv2) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {

	v.BaseLayer = BaseLayer{Contents: data[:len(data)]}
	v.Version = data[0] >> 4 // high nibble == VRRP version. We're expecting v2

	v.Type = VRRPv2Type(data[0] & 0x0F) // low nibble == VRRP type. Expecting 1 (advertisement)
	if v.Type != 1 {
		// rfc3768: A packet with unknown type MUST be discarded.
		return errors.New("Unrecognized VRRPv2 type field.")
	}

	v.VirtualRtrID = data[1]
	v.Priority = data[2]

	v.CountIPAddr = data[3]
	if v.CountIPAddr < 1 {
		return errors.New("VRRPv2 number of IP addresses is not valid.")
	}

	v.AuthType = VRRPv2AuthType(data[4])
	v.AdverInt = uint8(data[5])
	v.Checksum = binary.BigEndian.Uint16(data[6:8])

	// populate the IPAddress field. The number of addresses is specified in the v.CountIPAddr field
	// offset references the starting byte containing the list of ip addresses
	offset := 8
	for i := uint8(0); i < v.CountIPAddr; i++ {
		v.IPAddress = append(v.IPAddress, data[offset:offset+4])
		offset += 4
	}

	//	any trailing packets here may be authentication data and *should* be ignored in v2 as per RFC
	//
	//			5.3.10.  Authentication Data
	//
	//			The authentication string is currently only used to maintain
	//			backwards compatibility with RFC 2338.  It SHOULD be set to zero on
	//	   		transmission and ignored on reception.
	return nil
}

// CanDecode specifies the layer type in which we are attempting to unwrap.
func (v *VRRPv2) CanDecode() gopacket.LayerClass {
	return LayerTypeVRRP
}

// NextLayerType specifies the next layer that should be decoded. VRRP does not contain any further payload, so we set to 0
func (v *VRRPv2) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypeZero
}

// The VRRP packet does not include payload data. Setting byte slice to nil
func (v *VRRPv2) Payload() []byte {
	return nil
}

// decodeVRRP will parse VRRP v2
func decodeVRRP(data []byte, p gopacket.PacketBuilder) error {
	if len(data) < 8 {
		return errors.New("Not a valid VRRP packet. Packet length is too small.")
	}
	v := &VRRPv2{}
	return decodingLayerDecoder(v, data, p)
}
