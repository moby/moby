// Copyright 2020 The GoPacket Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file in the root of the source tree.

package layers

import (
	"encoding/binary"
	"fmt"

	"github.com/google/gopacket"
)

const (
	// RFC 2865 3.  Packet Format
	// `The minimum length is 20 and maximum length is 4096.`
	radiusMinimumRecordSizeInBytes int = 20
	radiusMaximumRecordSizeInBytes int = 4096

	// RFC 2865 5.  Attributes
	// `The Length field is one octet, and indicates the length of this Attribute including the Type, Length and Value fields.`
	// `The Value field is zero or more octets and contains information specific to the Attribute.`
	radiusAttributesMinimumRecordSizeInBytes int = 2
)

// RADIUS represents a Remote Authentication Dial In User Service layer.
type RADIUS struct {
	BaseLayer

	Code          RADIUSCode
	Identifier    RADIUSIdentifier
	Length        RADIUSLength
	Authenticator RADIUSAuthenticator
	Attributes    []RADIUSAttribute
}

// RADIUSCode represents packet type.
type RADIUSCode uint8

// constants that define RADIUSCode.
const (
	RADIUSCodeAccessRequest      RADIUSCode = 1   // RFC2865 3.  Packet Format
	RADIUSCodeAccessAccept       RADIUSCode = 2   // RFC2865 3.  Packet Format
	RADIUSCodeAccessReject       RADIUSCode = 3   // RFC2865 3.  Packet Format
	RADIUSCodeAccountingRequest  RADIUSCode = 4   // RFC2865 3.  Packet Format
	RADIUSCodeAccountingResponse RADIUSCode = 5   // RFC2865 3.  Packet Format
	RADIUSCodeAccessChallenge    RADIUSCode = 11  // RFC2865 3.  Packet Format
	RADIUSCodeStatusServer       RADIUSCode = 12  // RFC2865 3.  Packet Format (experimental)
	RADIUSCodeStatusClient       RADIUSCode = 13  // RFC2865 3.  Packet Format (experimental)
	RADIUSCodeReserved           RADIUSCode = 255 // RFC2865 3.  Packet Format
)

// String returns a string version of a RADIUSCode.
func (t RADIUSCode) String() (s string) {
	switch t {
	case RADIUSCodeAccessRequest:
		s = "Access-Request"
	case RADIUSCodeAccessAccept:
		s = "Access-Accept"
	case RADIUSCodeAccessReject:
		s = "Access-Reject"
	case RADIUSCodeAccountingRequest:
		s = "Accounting-Request"
	case RADIUSCodeAccountingResponse:
		s = "Accounting-Response"
	case RADIUSCodeAccessChallenge:
		s = "Access-Challenge"
	case RADIUSCodeStatusServer:
		s = "Status-Server"
	case RADIUSCodeStatusClient:
		s = "Status-Client"
	case RADIUSCodeReserved:
		s = "Reserved"
	default:
		s = fmt.Sprintf("Unknown(%d)", t)
	}
	return
}

// RADIUSIdentifier represents packet identifier.
type RADIUSIdentifier uint8

// RADIUSLength represents packet length.
type RADIUSLength uint16

// RADIUSAuthenticator represents authenticator.
type RADIUSAuthenticator [16]byte

// RADIUSAttribute represents attributes.
type RADIUSAttribute struct {
	Type   RADIUSAttributeType
	Length RADIUSAttributeLength
	Value  RADIUSAttributeValue
}

// RADIUSAttributeType represents attribute type.
type RADIUSAttributeType uint8

// constants that define RADIUSAttributeType.
const (
	RADIUSAttributeTypeUserName               RADIUSAttributeType = 1  // RFC2865  5.1.  User-Name
	RADIUSAttributeTypeUserPassword           RADIUSAttributeType = 2  // RFC2865  5.2.  User-Password
	RADIUSAttributeTypeCHAPPassword           RADIUSAttributeType = 3  // RFC2865  5.3.  CHAP-Password
	RADIUSAttributeTypeNASIPAddress           RADIUSAttributeType = 4  // RFC2865  5.4.  NAS-IP-Address
	RADIUSAttributeTypeNASPort                RADIUSAttributeType = 5  // RFC2865  5.5.  NAS-Port
	RADIUSAttributeTypeServiceType            RADIUSAttributeType = 6  // RFC2865  5.6.  Service-Type
	RADIUSAttributeTypeFramedProtocol         RADIUSAttributeType = 7  // RFC2865  5.7.  Framed-Protocol
	RADIUSAttributeTypeFramedIPAddress        RADIUSAttributeType = 8  // RFC2865  5.8.  Framed-IP-Address
	RADIUSAttributeTypeFramedIPNetmask        RADIUSAttributeType = 9  // RFC2865  5.9.  Framed-IP-Netmask
	RADIUSAttributeTypeFramedRouting          RADIUSAttributeType = 10 // RFC2865 5.10.  Framed-Routing
	RADIUSAttributeTypeFilterId               RADIUSAttributeType = 11 // RFC2865 5.11.  Filter-Id
	RADIUSAttributeTypeFramedMTU              RADIUSAttributeType = 12 // RFC2865 5.12.  Framed-MTU
	RADIUSAttributeTypeFramedCompression      RADIUSAttributeType = 13 // RFC2865 5.13.  Framed-Compression
	RADIUSAttributeTypeLoginIPHost            RADIUSAttributeType = 14 // RFC2865 5.14.  Login-IP-Host
	RADIUSAttributeTypeLoginService           RADIUSAttributeType = 15 // RFC2865 5.15.  Login-Service
	RADIUSAttributeTypeLoginTCPPort           RADIUSAttributeType = 16 // RFC2865 5.16.  Login-TCP-Port
	RADIUSAttributeTypeReplyMessage           RADIUSAttributeType = 18 // RFC2865 5.18.  Reply-Message
	RADIUSAttributeTypeCallbackNumber         RADIUSAttributeType = 19 // RFC2865 5.19.  Callback-Number
	RADIUSAttributeTypeCallbackId             RADIUSAttributeType = 20 // RFC2865 5.20.  Callback-Id
	RADIUSAttributeTypeFramedRoute            RADIUSAttributeType = 22 // RFC2865 5.22.  Framed-Route
	RADIUSAttributeTypeFramedIPXNetwork       RADIUSAttributeType = 23 // RFC2865 5.23.  Framed-IPX-Network
	RADIUSAttributeTypeState                  RADIUSAttributeType = 24 // RFC2865 5.24.  State
	RADIUSAttributeTypeClass                  RADIUSAttributeType = 25 // RFC2865 5.25.  Class
	RADIUSAttributeTypeVendorSpecific         RADIUSAttributeType = 26 // RFC2865 5.26.  Vendor-Specific
	RADIUSAttributeTypeSessionTimeout         RADIUSAttributeType = 27 // RFC2865 5.27.  Session-Timeout
	RADIUSAttributeTypeIdleTimeout            RADIUSAttributeType = 28 // RFC2865 5.28.  Idle-Timeout
	RADIUSAttributeTypeTerminationAction      RADIUSAttributeType = 29 // RFC2865 5.29.  Termination-Action
	RADIUSAttributeTypeCalledStationId        RADIUSAttributeType = 30 // RFC2865 5.30.  Called-Station-Id
	RADIUSAttributeTypeCallingStationId       RADIUSAttributeType = 31 // RFC2865 5.31.  Calling-Station-Id
	RADIUSAttributeTypeNASIdentifier          RADIUSAttributeType = 32 // RFC2865 5.32.  NAS-Identifier
	RADIUSAttributeTypeProxyState             RADIUSAttributeType = 33 // RFC2865 5.33.  Proxy-State
	RADIUSAttributeTypeLoginLATService        RADIUSAttributeType = 34 // RFC2865 5.34.  Login-LAT-Service
	RADIUSAttributeTypeLoginLATNode           RADIUSAttributeType = 35 // RFC2865 5.35.  Login-LAT-Node
	RADIUSAttributeTypeLoginLATGroup          RADIUSAttributeType = 36 // RFC2865 5.36.  Login-LAT-Group
	RADIUSAttributeTypeFramedAppleTalkLink    RADIUSAttributeType = 37 // RFC2865 5.37.  Framed-AppleTalk-Link
	RADIUSAttributeTypeFramedAppleTalkNetwork RADIUSAttributeType = 38 // RFC2865 5.38.  Framed-AppleTalk-Network
	RADIUSAttributeTypeFramedAppleTalkZone    RADIUSAttributeType = 39 // RFC2865 5.39.  Framed-AppleTalk-Zone
	RADIUSAttributeTypeAcctStatusType         RADIUSAttributeType = 40 // RFC2866  5.1.  Acct-Status-Type
	RADIUSAttributeTypeAcctDelayTime          RADIUSAttributeType = 41 // RFC2866  5.2.  Acct-Delay-Time
	RADIUSAttributeTypeAcctInputOctets        RADIUSAttributeType = 42 // RFC2866  5.3.  Acct-Input-Octets
	RADIUSAttributeTypeAcctOutputOctets       RADIUSAttributeType = 43 // RFC2866  5.4.  Acct-Output-Octets
	RADIUSAttributeTypeAcctSessionId          RADIUSAttributeType = 44 // RFC2866  5.5.  Acct-Session-Id
	RADIUSAttributeTypeAcctAuthentic          RADIUSAttributeType = 45 // RFC2866  5.6.  Acct-Authentic
	RADIUSAttributeTypeAcctSessionTime        RADIUSAttributeType = 46 // RFC2866  5.7.  Acct-Session-Time
	RADIUSAttributeTypeAcctInputPackets       RADIUSAttributeType = 47 // RFC2866  5.8.  Acct-Input-Packets
	RADIUSAttributeTypeAcctOutputPackets      RADIUSAttributeType = 48 // RFC2866  5.9.  Acct-Output-Packets
	RADIUSAttributeTypeAcctTerminateCause     RADIUSAttributeType = 49 // RFC2866 5.10.  Acct-Terminate-Cause
	RADIUSAttributeTypeAcctMultiSessionId     RADIUSAttributeType = 50 // RFC2866 5.11.  Acct-Multi-Session-Id
	RADIUSAttributeTypeAcctLinkCount          RADIUSAttributeType = 51 // RFC2866 5.12.  Acct-Link-Count
	RADIUSAttributeTypeAcctInputGigawords     RADIUSAttributeType = 52 // RFC2869  5.1.  Acct-Input-Gigawords
	RADIUSAttributeTypeAcctOutputGigawords    RADIUSAttributeType = 53 // RFC2869  5.2.  Acct-Output-Gigawords
	RADIUSAttributeTypeEventTimestamp         RADIUSAttributeType = 55 // RFC2869  5.3.  Event-Timestamp
	RADIUSAttributeTypeCHAPChallenge          RADIUSAttributeType = 60 // RFC2865 5.40.  CHAP-Challenge
	RADIUSAttributeTypeNASPortType            RADIUSAttributeType = 61 // RFC2865 5.41.  NAS-Port-Type
	RADIUSAttributeTypePortLimit              RADIUSAttributeType = 62 // RFC2865 5.42.  Port-Limit
	RADIUSAttributeTypeLoginLATPort           RADIUSAttributeType = 63 // RFC2865 5.43.  Login-LAT-Port
	RADIUSAttributeTypeTunnelType             RADIUSAttributeType = 64 // RFC2868  3.1.  Tunnel-Type
	RADIUSAttributeTypeTunnelMediumType       RADIUSAttributeType = 65 // RFC2868  3.2.  Tunnel-Medium-Type
	RADIUSAttributeTypeTunnelClientEndpoint   RADIUSAttributeType = 66 // RFC2868  3.3.  Tunnel-Client-Endpoint
	RADIUSAttributeTypeTunnelServerEndpoint   RADIUSAttributeType = 67 // RFC2868  3.4.  Tunnel-Server-Endpoint
	RADIUSAttributeTypeAcctTunnelConnection   RADIUSAttributeType = 68 // RFC2867  4.1.  Acct-Tunnel-Connection
	RADIUSAttributeTypeTunnelPassword         RADIUSAttributeType = 69 // RFC2868  3.5.  Tunnel-Password
	RADIUSAttributeTypeARAPPassword           RADIUSAttributeType = 70 // RFC2869  5.4.  ARAP-Password
	RADIUSAttributeTypeARAPFeatures           RADIUSAttributeType = 71 // RFC2869  5.5.  ARAP-Features
	RADIUSAttributeTypeARAPZoneAccess         RADIUSAttributeType = 72 // RFC2869  5.6.  ARAP-Zone-Access
	RADIUSAttributeTypeARAPSecurity           RADIUSAttributeType = 73 // RFC2869  5.7.  ARAP-Security
	RADIUSAttributeTypeARAPSecurityData       RADIUSAttributeType = 74 // RFC2869  5.8.  ARAP-Security-Data
	RADIUSAttributeTypePasswordRetry          RADIUSAttributeType = 75 // RFC2869  5.9.  Password-Retry
	RADIUSAttributeTypePrompt                 RADIUSAttributeType = 76 // RFC2869 5.10.  Prompt
	RADIUSAttributeTypeConnectInfo            RADIUSAttributeType = 77 // RFC2869 5.11.  Connect-Info
	RADIUSAttributeTypeConfigurationToken     RADIUSAttributeType = 78 // RFC2869 5.12.  Configuration-Token
	RADIUSAttributeTypeEAPMessage             RADIUSAttributeType = 79 // RFC2869 5.13.  EAP-Message
	RADIUSAttributeTypeMessageAuthenticator   RADIUSAttributeType = 80 // RFC2869 5.14.  Message-Authenticator
	RADIUSAttributeTypeTunnelPrivateGroupID   RADIUSAttributeType = 81 // RFC2868  3.6.  Tunnel-Private-Group-ID
	RADIUSAttributeTypeTunnelAssignmentID     RADIUSAttributeType = 82 // RFC2868  3.7.  Tunnel-Assignment-ID
	RADIUSAttributeTypeTunnelPreference       RADIUSAttributeType = 83 // RFC2868  3.8.  Tunnel-Preference
	RADIUSAttributeTypeARAPChallengeResponse  RADIUSAttributeType = 84 // RFC2869 5.15.  ARAP-Challenge-Response
	RADIUSAttributeTypeAcctInterimInterval    RADIUSAttributeType = 85 // RFC2869 5.16.  Acct-Interim-Interval
	RADIUSAttributeTypeAcctTunnelPacketsLost  RADIUSAttributeType = 86 // RFC2867  4.2.  Acct-Tunnel-Packets-Lost
	RADIUSAttributeTypeNASPortId              RADIUSAttributeType = 87 // RFC2869 5.17.  NAS-Port-Id
	RADIUSAttributeTypeFramedPool             RADIUSAttributeType = 88 // RFC2869 5.18.  Framed-Pool
	RADIUSAttributeTypeTunnelClientAuthID     RADIUSAttributeType = 90 // RFC2868  3.9.  Tunnel-Client-Auth-ID
	RADIUSAttributeTypeTunnelServerAuthID     RADIUSAttributeType = 91 // RFC2868 3.10.  Tunnel-Server-Auth-ID
)

// RADIUSAttributeType represents attribute length.
type RADIUSAttributeLength uint8

// RADIUSAttributeType represents attribute value.
type RADIUSAttributeValue []byte

// String returns a string version of a RADIUSAttributeType.
func (t RADIUSAttributeType) String() (s string) {
	switch t {
	case RADIUSAttributeTypeUserName:
		s = "User-Name"
	case RADIUSAttributeTypeUserPassword:
		s = "User-Password"
	case RADIUSAttributeTypeCHAPPassword:
		s = "CHAP-Password"
	case RADIUSAttributeTypeNASIPAddress:
		s = "NAS-IP-Address"
	case RADIUSAttributeTypeNASPort:
		s = "NAS-Port"
	case RADIUSAttributeTypeServiceType:
		s = "Service-Type"
	case RADIUSAttributeTypeFramedProtocol:
		s = "Framed-Protocol"
	case RADIUSAttributeTypeFramedIPAddress:
		s = "Framed-IP-Address"
	case RADIUSAttributeTypeFramedIPNetmask:
		s = "Framed-IP-Netmask"
	case RADIUSAttributeTypeFramedRouting:
		s = "Framed-Routing"
	case RADIUSAttributeTypeFilterId:
		s = "Filter-Id"
	case RADIUSAttributeTypeFramedMTU:
		s = "Framed-MTU"
	case RADIUSAttributeTypeFramedCompression:
		s = "Framed-Compression"
	case RADIUSAttributeTypeLoginIPHost:
		s = "Login-IP-Host"
	case RADIUSAttributeTypeLoginService:
		s = "Login-Service"
	case RADIUSAttributeTypeLoginTCPPort:
		s = "Login-TCP-Port"
	case RADIUSAttributeTypeReplyMessage:
		s = "Reply-Message"
	case RADIUSAttributeTypeCallbackNumber:
		s = "Callback-Number"
	case RADIUSAttributeTypeCallbackId:
		s = "Callback-Id"
	case RADIUSAttributeTypeFramedRoute:
		s = "Framed-Route"
	case RADIUSAttributeTypeFramedIPXNetwork:
		s = "Framed-IPX-Network"
	case RADIUSAttributeTypeState:
		s = "State"
	case RADIUSAttributeTypeClass:
		s = "Class"
	case RADIUSAttributeTypeVendorSpecific:
		s = "Vendor-Specific"
	case RADIUSAttributeTypeSessionTimeout:
		s = "Session-Timeout"
	case RADIUSAttributeTypeIdleTimeout:
		s = "Idle-Timeout"
	case RADIUSAttributeTypeTerminationAction:
		s = "Termination-Action"
	case RADIUSAttributeTypeCalledStationId:
		s = "Called-Station-Id"
	case RADIUSAttributeTypeCallingStationId:
		s = "Calling-Station-Id"
	case RADIUSAttributeTypeNASIdentifier:
		s = "NAS-Identifier"
	case RADIUSAttributeTypeProxyState:
		s = "Proxy-State"
	case RADIUSAttributeTypeLoginLATService:
		s = "Login-LAT-Service"
	case RADIUSAttributeTypeLoginLATNode:
		s = "Login-LAT-Node"
	case RADIUSAttributeTypeLoginLATGroup:
		s = "Login-LAT-Group"
	case RADIUSAttributeTypeFramedAppleTalkLink:
		s = "Framed-AppleTalk-Link"
	case RADIUSAttributeTypeFramedAppleTalkNetwork:
		s = "Framed-AppleTalk-Network"
	case RADIUSAttributeTypeFramedAppleTalkZone:
		s = "Framed-AppleTalk-Zone"
	case RADIUSAttributeTypeAcctStatusType:
		s = "Acct-Status-Type"
	case RADIUSAttributeTypeAcctDelayTime:
		s = "Acct-Delay-Time"
	case RADIUSAttributeTypeAcctInputOctets:
		s = "Acct-Input-Octets"
	case RADIUSAttributeTypeAcctOutputOctets:
		s = "Acct-Output-Octets"
	case RADIUSAttributeTypeAcctSessionId:
		s = "Acct-Session-Id"
	case RADIUSAttributeTypeAcctAuthentic:
		s = "Acct-Authentic"
	case RADIUSAttributeTypeAcctSessionTime:
		s = "Acct-Session-Time"
	case RADIUSAttributeTypeAcctInputPackets:
		s = "Acct-Input-Packets"
	case RADIUSAttributeTypeAcctOutputPackets:
		s = "Acct-Output-Packets"
	case RADIUSAttributeTypeAcctTerminateCause:
		s = "Acct-Terminate-Cause"
	case RADIUSAttributeTypeAcctMultiSessionId:
		s = "Acct-Multi-Session-Id"
	case RADIUSAttributeTypeAcctLinkCount:
		s = "Acct-Link-Count"
	case RADIUSAttributeTypeAcctInputGigawords:
		s = "Acct-Input-Gigawords"
	case RADIUSAttributeTypeAcctOutputGigawords:
		s = "Acct-Output-Gigawords"
	case RADIUSAttributeTypeEventTimestamp:
		s = "Event-Timestamp"
	case RADIUSAttributeTypeCHAPChallenge:
		s = "CHAP-Challenge"
	case RADIUSAttributeTypeNASPortType:
		s = "NAS-Port-Type"
	case RADIUSAttributeTypePortLimit:
		s = "Port-Limit"
	case RADIUSAttributeTypeLoginLATPort:
		s = "Login-LAT-Port"
	case RADIUSAttributeTypeTunnelType:
		s = "Tunnel-Type"
	case RADIUSAttributeTypeTunnelMediumType:
		s = "Tunnel-Medium-Type"
	case RADIUSAttributeTypeTunnelClientEndpoint:
		s = "Tunnel-Client-Endpoint"
	case RADIUSAttributeTypeTunnelServerEndpoint:
		s = "Tunnel-Server-Endpoint"
	case RADIUSAttributeTypeAcctTunnelConnection:
		s = "Acct-Tunnel-Connection"
	case RADIUSAttributeTypeTunnelPassword:
		s = "Tunnel-Password"
	case RADIUSAttributeTypeARAPPassword:
		s = "ARAP-Password"
	case RADIUSAttributeTypeARAPFeatures:
		s = "ARAP-Features"
	case RADIUSAttributeTypeARAPZoneAccess:
		s = "ARAP-Zone-Access"
	case RADIUSAttributeTypeARAPSecurity:
		s = "ARAP-Security"
	case RADIUSAttributeTypeARAPSecurityData:
		s = "ARAP-Security-Data"
	case RADIUSAttributeTypePasswordRetry:
		s = "Password-Retry"
	case RADIUSAttributeTypePrompt:
		s = "Prompt"
	case RADIUSAttributeTypeConnectInfo:
		s = "Connect-Info"
	case RADIUSAttributeTypeConfigurationToken:
		s = "Configuration-Token"
	case RADIUSAttributeTypeEAPMessage:
		s = "EAP-Message"
	case RADIUSAttributeTypeMessageAuthenticator:
		s = "Message-Authenticator"
	case RADIUSAttributeTypeTunnelPrivateGroupID:
		s = "Tunnel-Private-Group-ID"
	case RADIUSAttributeTypeTunnelAssignmentID:
		s = "Tunnel-Assignment-ID"
	case RADIUSAttributeTypeTunnelPreference:
		s = "Tunnel-Preference"
	case RADIUSAttributeTypeARAPChallengeResponse:
		s = "ARAP-Challenge-Response"
	case RADIUSAttributeTypeAcctInterimInterval:
		s = "Acct-Interim-Interval"
	case RADIUSAttributeTypeAcctTunnelPacketsLost:
		s = "Acct-Tunnel-Packets-Lost"
	case RADIUSAttributeTypeNASPortId:
		s = "NAS-Port-Id"
	case RADIUSAttributeTypeFramedPool:
		s = "Framed-Pool"
	case RADIUSAttributeTypeTunnelClientAuthID:
		s = "Tunnel-Client-Auth-ID"
	case RADIUSAttributeTypeTunnelServerAuthID:
		s = "Tunnel-Server-Auth-ID"
	default:
		s = fmt.Sprintf("Unknown(%d)", t)
	}
	return
}

// Len returns the length of a RADIUS packet.
func (radius *RADIUS) Len() (int, error) {
	n := radiusMinimumRecordSizeInBytes
	for _, v := range radius.Attributes {
		alen, err := attributeValueLength(v.Value)
		if err != nil {
			return 0, err
		}
		n += int(alen) + 2 // Added Type and Length
	}
	return n, nil
}

// LayerType returns LayerTypeRADIUS.
func (radius *RADIUS) LayerType() gopacket.LayerType {
	return LayerTypeRADIUS
}

// DecodeFromBytes decodes the given bytes into this layer.
func (radius *RADIUS) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) > radiusMaximumRecordSizeInBytes {
		df.SetTruncated()
		return fmt.Errorf("RADIUS length %d too big", len(data))
	}

	if len(data) < radiusMinimumRecordSizeInBytes {
		df.SetTruncated()
		return fmt.Errorf("RADIUS length %d too short", len(data))
	}

	radius.BaseLayer = BaseLayer{Contents: data}

	radius.Code = RADIUSCode(data[0])
	radius.Identifier = RADIUSIdentifier(data[1])
	radius.Length = RADIUSLength(binary.BigEndian.Uint16(data[2:4]))

	if int(radius.Length) > radiusMaximumRecordSizeInBytes {
		df.SetTruncated()
		return fmt.Errorf("RADIUS length %d too big", radius.Length)
	}

	if int(radius.Length) < radiusMinimumRecordSizeInBytes {
		df.SetTruncated()
		return fmt.Errorf("RADIUS length %d too short", radius.Length)
	}

	// RFC 2865 3.  Packet Format
	// `If the packet is shorter than the Length field indicates, it MUST be silently discarded.`
	if int(radius.Length) > len(data) {
		df.SetTruncated()
		return fmt.Errorf("RADIUS length %d too big", radius.Length)
	}

	// RFC 2865 3.  Packet Format
	// `Octets outside the range of the Length field MUST be treated as padding and ignored on reception.`
	if int(radius.Length) < len(data) {
		df.SetTruncated()
		data = data[:radius.Length]
	}

	copy(radius.Authenticator[:], data[4:20])

	if len(data) == radiusMinimumRecordSizeInBytes {
		return nil
	}

	pos := radiusMinimumRecordSizeInBytes
	for {
		if len(data) == pos {
			break
		}

		if len(data[pos:]) < radiusAttributesMinimumRecordSizeInBytes {
			df.SetTruncated()
			return fmt.Errorf("RADIUS attributes length %d too short", len(data[pos:]))
		}

		attr := RADIUSAttribute{}
		attr.Type = RADIUSAttributeType(data[pos])
		attr.Length = RADIUSAttributeLength(data[pos+1])

		if int(attr.Length) > len(data[pos:]) {
			df.SetTruncated()
			return fmt.Errorf("RADIUS attributes length %d too big", attr.Length)
		}

		if int(attr.Length) < radiusAttributesMinimumRecordSizeInBytes {
			df.SetTruncated()
			return fmt.Errorf("RADIUS attributes length %d too short", attr.Length)
		}

		if int(attr.Length) > radiusAttributesMinimumRecordSizeInBytes {
			attr.Value = make([]byte, attr.Length-2)
			copy(attr.Value[:], data[pos+2:pos+int(attr.Length)])
			radius.Attributes = append(radius.Attributes, attr)
		}

		pos += int(attr.Length)
	}

	for _, v := range radius.Attributes {
		if v.Type == RADIUSAttributeTypeEAPMessage {
			radius.BaseLayer.Payload = append(radius.BaseLayer.Payload, v.Value...)
		}
	}

	return nil
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (radius *RADIUS) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	plen, err := radius.Len()
	if err != nil {
		return err
	}

	if opts.FixLengths {
		radius.Length = RADIUSLength(plen)
	}

	data, err := b.PrependBytes(plen)
	if err != nil {
		return err
	}

	data[0] = byte(radius.Code)
	data[1] = byte(radius.Identifier)
	binary.BigEndian.PutUint16(data[2:], uint16(radius.Length))
	copy(data[4:20], radius.Authenticator[:])

	pos := radiusMinimumRecordSizeInBytes
	for _, v := range radius.Attributes {
		if opts.FixLengths {
			v.Length, err = attributeValueLength(v.Value)
			if err != nil {
				return err
			}
		}

		data[pos] = byte(v.Type)
		data[pos+1] = byte(v.Length)
		copy(data[pos+2:], v.Value[:])

		pos += len(v.Value) + 2 // Added Type and Length
	}

	return nil
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (radius *RADIUS) CanDecode() gopacket.LayerClass {
	return LayerTypeRADIUS
}

// NextLayerType returns the layer type contained by this DecodingLayer.
func (radius *RADIUS) NextLayerType() gopacket.LayerType {
	if len(radius.BaseLayer.Payload) > 0 {
		return LayerTypeEAP
	} else {
		return gopacket.LayerTypeZero
	}
}

// Payload returns the EAP Type-Data for EAP-Message attributes.
func (radius *RADIUS) Payload() []byte {
	return radius.BaseLayer.Payload
}

func decodeRADIUS(data []byte, p gopacket.PacketBuilder) error {
	radius := &RADIUS{}
	err := radius.DecodeFromBytes(data, p)
	if err != nil {
		return err
	}
	p.AddLayer(radius)
	p.SetApplicationLayer(radius)
	next := radius.NextLayerType()
	if next == gopacket.LayerTypeZero {
		return nil
	}
	return p.NextDecoder(next)
}

func attributeValueLength(v []byte) (RADIUSAttributeLength, error) {
	n := len(v)
	if n > 255 {
		return 0, fmt.Errorf("RADIUS attribute value length %d too long", n)
	} else {
		return RADIUSAttributeLength(n), nil
	}
}
