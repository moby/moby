// Copyright 2017 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.
//

package layers

import (
	"encoding/binary"
	"errors"

	"github.com/google/gopacket"
)

// BFD Control Packet Format
// -------------------------
// The current version of BFD's RFC (RFC 5880) contains the following
// diagram for the BFD Control packet format:
//
//      0                   1                   2                   3
//      0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//     |Vers |  Diag   |Sta|P|F|C|A|D|M|  Detect Mult  |    Length     |
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//     |                       My Discriminator                        |
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//     |                      Your Discriminator                       |
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//     |                    Desired Min TX Interval                    |
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//     |                   Required Min RX Interval                    |
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//     |                 Required Min Echo RX Interval                 |
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
//     An optional Authentication Section MAY be present:
//
//      0                   1                   2                   3
//      0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//     |   Auth Type   |   Auth Len    |    Authentication Data...     |
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
//
//     Simple Password Authentication Section Format
//     ---------------------------------------------
//      0                   1                   2                   3
//      0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//     |   Auth Type   |   Auth Len    |  Auth Key ID  |  Password...  |
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//     |                              ...                              |
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
//
//     Keyed MD5 and Meticulous Keyed MD5 Authentication Section Format
//     ----------------------------------------------------------------
//      0                   1                   2                   3
//      0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//     |   Auth Type   |   Auth Len    |  Auth Key ID  |   Reserved    |
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//     |                        Sequence Number                        |
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//     |                      Auth Key/Digest...                       |
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//     |                              ...                              |
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
//
//     Keyed SHA1 and Meticulous Keyed SHA1 Authentication Section Format
//     ------------------------------------------------------------------
//      0                   1                   2                   3
//      0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//     |   Auth Type   |   Auth Len    |  Auth Key ID  |   Reserved    |
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//     |                        Sequence Number                        |
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//     |                       Auth Key/Hash...                        |
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//     |                              ...                              |
//     +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
//     From https://tools.ietf.org/rfc/rfc5880.txt
const bfdMinimumRecordSizeInBytes int = 24

// BFDVersion represents the version as decoded from the BFD control message
type BFDVersion uint8

// BFDDiagnostic represents diagnostic infomation about a BFD session
type BFDDiagnostic uint8

// constants that define BFDDiagnostic flags
const (
	BFDDiagnosticNone               BFDDiagnostic = 0 // No Diagnostic
	BFDDiagnosticTimeExpired        BFDDiagnostic = 1 // Control Detection Time Expired
	BFDDiagnosticEchoFailed         BFDDiagnostic = 2 // Echo Function Failed
	BFDDiagnosticNeighborSignalDown BFDDiagnostic = 3 // Neighbor Signaled Session Down
	BFDDiagnosticForwardPlaneReset  BFDDiagnostic = 4 // Forwarding Plane Reset
	BFDDiagnosticPathDown           BFDDiagnostic = 5 // Path Down
	BFDDiagnosticConcatPathDown     BFDDiagnostic = 6 // Concatenated Path Down
	BFDDiagnosticAdminDown          BFDDiagnostic = 7 // Administratively Down
	BFDDiagnosticRevConcatPathDown  BFDDiagnostic = 8 // Reverse Concatenated Path Dow
)

// String returns a string version of BFDDiagnostic
func (bd BFDDiagnostic) String() string {
	switch bd {
	default:
		return "Unknown"
	case BFDDiagnosticNone:
		return "None"
	case BFDDiagnosticTimeExpired:
		return "Control Detection Time Expired"
	case BFDDiagnosticEchoFailed:
		return "Echo Function Failed"
	case BFDDiagnosticNeighborSignalDown:
		return "Neighbor Signaled Session Down"
	case BFDDiagnosticForwardPlaneReset:
		return "Forwarding Plane Reset"
	case BFDDiagnosticPathDown:
		return "Path Down"
	case BFDDiagnosticConcatPathDown:
		return "Concatenated Path Down"
	case BFDDiagnosticAdminDown:
		return "Administratively Down"
	case BFDDiagnosticRevConcatPathDown:
		return "Reverse Concatenated Path Down"
	}
}

// BFDState represents the state of a BFD session
type BFDState uint8

// constants that define BFDState
const (
	BFDStateAdminDown BFDState = 0
	BFDStateDown      BFDState = 1
	BFDStateInit      BFDState = 2
	BFDStateUp        BFDState = 3
)

// String returns a string version of BFDState
func (s BFDState) String() string {
	switch s {
	default:
		return "Unknown"
	case BFDStateAdminDown:
		return "Admin Down"
	case BFDStateDown:
		return "Down"
	case BFDStateInit:
		return "Init"
	case BFDStateUp:
		return "Up"
	}
}

// BFDDetectMultiplier represents the negotiated transmit interval,
// multiplied by this value, provides the Detection Time for the
// receiving system in Asynchronous mode.
type BFDDetectMultiplier uint8

// BFDDiscriminator is a unique, nonzero discriminator value used
// to demultiplex multiple BFD sessions between the same pair of systems.
type BFDDiscriminator uint32

// BFDTimeInterval represents a time interval in microseconds
type BFDTimeInterval uint32

// BFDAuthType represents the authentication used in the BFD session
type BFDAuthType uint8

// constants that define the BFDAuthType
const (
	BFDAuthTypeNone                BFDAuthType = 0 // No Auth
	BFDAuthTypePassword            BFDAuthType = 1 // Simple Password
	BFDAuthTypeKeyedMD5            BFDAuthType = 2 // Keyed MD5
	BFDAuthTypeMeticulousKeyedMD5  BFDAuthType = 3 // Meticulous Keyed MD5
	BFDAuthTypeKeyedSHA1           BFDAuthType = 4 // Keyed SHA1
	BFDAuthTypeMeticulousKeyedSHA1 BFDAuthType = 5 // Meticulous Keyed SHA1
)

// String returns a string version of BFDAuthType
func (at BFDAuthType) String() string {
	switch at {
	default:
		return "Unknown"
	case BFDAuthTypeNone:
		return "No Authentication"
	case BFDAuthTypePassword:
		return "Simple Password"
	case BFDAuthTypeKeyedMD5:
		return "Keyed MD5"
	case BFDAuthTypeMeticulousKeyedMD5:
		return "Meticulous Keyed MD5"
	case BFDAuthTypeKeyedSHA1:
		return "Keyed SHA1"
	case BFDAuthTypeMeticulousKeyedSHA1:
		return "Meticulous Keyed SHA1"
	}
}

// BFDAuthKeyID represents the authentication key ID in use for
// this packet.  This allows multiple keys to be active simultaneously.
type BFDAuthKeyID uint8

// BFDAuthSequenceNumber represents the sequence number for this packet.
// For Keyed Authentication, this value is incremented occasionally.  For
// Meticulous Keyed Authentication, this value is incremented for each
// successive packet transmitted for a session.  This provides protection
// against replay attacks.
type BFDAuthSequenceNumber uint32

// BFDAuthData represents the authentication key or digest
type BFDAuthData []byte

// BFDAuthHeader represents authentication data used in the BFD session
type BFDAuthHeader struct {
	AuthType       BFDAuthType
	KeyID          BFDAuthKeyID
	SequenceNumber BFDAuthSequenceNumber
	Data           BFDAuthData
}

// Length returns the data length of the BFDAuthHeader based on the
// authentication type
func (h *BFDAuthHeader) Length() int {
	switch h.AuthType {
	case BFDAuthTypePassword:
		return 3 + len(h.Data)
	case BFDAuthTypeKeyedMD5, BFDAuthTypeMeticulousKeyedMD5:
		return 8 + len(h.Data)
	case BFDAuthTypeKeyedSHA1, BFDAuthTypeMeticulousKeyedSHA1:
		return 8 + len(h.Data)
	default:
		return 0
	}
}

// BFD represents a BFD control message packet whose payload contains
// the control information required to for a BFD session.
//
// References
// ----------
//
// Wikipedia's BFD entry:
//     https://en.wikipedia.org/wiki/Bidirectional_Forwarding_Detection
//     This is the best place to get an overview of BFD.
//
// RFC 5880 "Bidirectional Forwarding Detection (BFD)" (2010)
//     https://tools.ietf.org/html/rfc5880
//     This is the original BFD specification.
//
// RFC 5881 "Bidirectional Forwarding Detection (BFD) for IPv4 and IPv6 (Single Hop)" (2010)
//     https://tools.ietf.org/html/rfc5881
//     Describes the use of the Bidirectional Forwarding Detection (BFD)
//     protocol over IPv4 and IPv6 for single IP hops.
type BFD struct {
	BaseLayer // Stores the packet bytes and payload bytes.

	Version                   BFDVersion          // Version of the BFD protocol.
	Diagnostic                BFDDiagnostic       // Diagnostic code for last state change
	State                     BFDState            // Current state
	Poll                      bool                // Requesting verification
	Final                     bool                // Responding to a received BFD Control packet that had the Poll (P) bit set.
	ControlPlaneIndependent   bool                // BFD implementation does not share fate with its control plane
	AuthPresent               bool                // Authentication Section is present and the session is to be authenticated
	Demand                    bool                // Demand mode is active
	Multipoint                bool                // For future point-to-multipoint extensions. Must always be zero
	DetectMultiplier          BFDDetectMultiplier // Detection time multiplier
	MyDiscriminator           BFDDiscriminator    // A unique, nonzero discriminator value
	YourDiscriminator         BFDDiscriminator    // discriminator received from the remote system.
	DesiredMinTxInterval      BFDTimeInterval     // Minimum interval, in microseconds,  the local system would like to use when transmitting BFD Control packets
	RequiredMinRxInterval     BFDTimeInterval     // Minimum interval, in microseconds, between received BFD Control packets that this system is capable of supporting
	RequiredMinEchoRxInterval BFDTimeInterval     // Minimum interval, in microseconds, between received BFD Echo packets that this system is capable of supporting
	AuthHeader                *BFDAuthHeader      // Authentication data, variable length.
}

// Length returns the data length of a BFD Control message which
// changes based on the presence and type of authentication
// contained in the message
func (d *BFD) Length() int {
	if d.AuthPresent && (d.AuthHeader != nil) {
		return bfdMinimumRecordSizeInBytes + d.AuthHeader.Length()
	}

	return bfdMinimumRecordSizeInBytes
}

// LayerType returns the layer type of the BFD object, which is LayerTypeBFD.
func (d *BFD) LayerType() gopacket.LayerType {
	return LayerTypeBFD
}

// decodeBFD analyses a byte slice and attempts to decode it as a BFD
// control packet
//
// If it succeeds, it loads p with information about the packet and returns nil.
// If it fails, it returns an error (non nil).
//
// This function is employed in layertypes.go to register the BFD layer.
func decodeBFD(data []byte, p gopacket.PacketBuilder) error {

	// Attempt to decode the byte slice.
	d := &BFD{}
	err := d.DecodeFromBytes(data, p)
	if err != nil {
		return err
	}

	// If the decoding worked, add the layer to the packet and set it
	// as the application layer too, if there isn't already one.
	p.AddLayer(d)
	p.SetApplicationLayer(d)

	return nil
}

// DecodeFromBytes analyses a byte slice and attempts to decode it as a BFD
// control packet.
//
// Upon succeeds, it loads the BFD object with information about the packet
// and returns nil.
// Upon failure, it returns an error (non nil).
func (d *BFD) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {

	// If the data block is too short to be a BFD record, then return an error.
	if len(data) < bfdMinimumRecordSizeInBytes {
		df.SetTruncated()
		return errors.New("BFD packet too short")
	}

	pLen := uint8(data[3])
	if len(data) != int(pLen) {
		return errors.New("BFD packet length does not match")
	}

	// BFD type embeds type BaseLayer which contains two fields:
	//    Contents is supposed to contain the bytes of the data at this level.
	//    Payload is supposed to contain the payload of this level.
	// Here we set the baselayer to be the bytes of the BFD record.
	d.BaseLayer = BaseLayer{Contents: data[:len(data)]}

	// Extract the fields from the block of bytes.
	// To make sense of this, refer to the packet diagram
	// above and the section on endian conventions.

	// The first few fields are all packed into the first 32 bits. Unpack them.
	d.Version = BFDVersion(((data[0] & 0xE0) >> 5))
	d.Diagnostic = BFDDiagnostic(data[0] & 0x1F)
	data = data[1:]

	d.State = BFDState((data[0] & 0xC0) >> 6)
	d.Poll = data[0]&0x20 != 0
	d.Final = data[0]&0x10 != 0
	d.ControlPlaneIndependent = data[0]&0x08 != 0
	d.AuthPresent = data[0]&0x04 != 0
	d.Demand = data[0]&0x02 != 0
	d.Multipoint = data[0]&0x01 != 0
	data = data[1:]

	data, d.DetectMultiplier = data[1:], BFDDetectMultiplier(data[0])
	data, _ = data[1:], uint8(data[0]) // Consume length

	// The remaining fields can just be copied in big endian order.
	data, d.MyDiscriminator = data[4:], BFDDiscriminator(binary.BigEndian.Uint32(data[:4]))
	data, d.YourDiscriminator = data[4:], BFDDiscriminator(binary.BigEndian.Uint32(data[:4]))
	data, d.DesiredMinTxInterval = data[4:], BFDTimeInterval(binary.BigEndian.Uint32(data[:4]))
	data, d.RequiredMinRxInterval = data[4:], BFDTimeInterval(binary.BigEndian.Uint32(data[:4]))
	data, d.RequiredMinEchoRxInterval = data[4:], BFDTimeInterval(binary.BigEndian.Uint32(data[:4]))

	if d.AuthPresent && (len(data) > 2) {
		d.AuthHeader = &BFDAuthHeader{}
		data, d.AuthHeader.AuthType = data[1:], BFDAuthType(data[0])
		data, _ = data[1:], uint8(data[0]) // Consume length
		data, d.AuthHeader.KeyID = data[1:], BFDAuthKeyID(data[0])

		switch d.AuthHeader.AuthType {
		case BFDAuthTypePassword:
			d.AuthHeader.Data = BFDAuthData(data)
		case BFDAuthTypeKeyedMD5, BFDAuthTypeMeticulousKeyedMD5:
			// Skipped reserved byte
			data, d.AuthHeader.SequenceNumber = data[5:], BFDAuthSequenceNumber(binary.BigEndian.Uint32(data[1:5]))
			d.AuthHeader.Data = BFDAuthData(data)
		case BFDAuthTypeKeyedSHA1, BFDAuthTypeMeticulousKeyedSHA1:
			// Skipped reserved byte
			data, d.AuthHeader.SequenceNumber = data[5:], BFDAuthSequenceNumber(binary.BigEndian.Uint32(data[1:5]))
			d.AuthHeader.Data = BFDAuthData(data)
		}
	}

	return nil
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (d *BFD) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	data, err := b.PrependBytes(bfdMinimumRecordSizeInBytes)
	if err != nil {
		return err
	}

	// Pack the first few fields into the first 32 bits.
	data[0] = byte(byte(d.Version<<5) | byte(d.Diagnostic))
	h := uint8(0)
	h |= (uint8(d.State) << 6)
	h |= (uint8(bool2uint8(d.Poll)) << 5)
	h |= (uint8(bool2uint8(d.Final)) << 4)
	h |= (uint8(bool2uint8(d.ControlPlaneIndependent)) << 3)
	h |= (uint8(bool2uint8(d.AuthPresent)) << 2)
	h |= (uint8(bool2uint8(d.Demand)) << 1)
	h |= uint8(bool2uint8(d.Multipoint))
	data[1] = byte(h)
	data[2] = byte(d.DetectMultiplier)
	data[3] = byte(d.Length())

	// The remaining fields can just be copied in big endian order.
	binary.BigEndian.PutUint32(data[4:], uint32(d.MyDiscriminator))
	binary.BigEndian.PutUint32(data[8:], uint32(d.YourDiscriminator))
	binary.BigEndian.PutUint32(data[12:], uint32(d.DesiredMinTxInterval))
	binary.BigEndian.PutUint32(data[16:], uint32(d.RequiredMinRxInterval))
	binary.BigEndian.PutUint32(data[20:], uint32(d.RequiredMinEchoRxInterval))

	if d.AuthPresent && (d.AuthHeader != nil) {
		auth, err := b.AppendBytes(int(d.AuthHeader.Length()))
		if err != nil {
			return err
		}

		auth[0] = byte(d.AuthHeader.AuthType)
		auth[1] = byte(d.AuthHeader.Length())
		auth[2] = byte(d.AuthHeader.KeyID)

		switch d.AuthHeader.AuthType {
		case BFDAuthTypePassword:
			copy(auth[3:], d.AuthHeader.Data)
		case BFDAuthTypeKeyedMD5, BFDAuthTypeMeticulousKeyedMD5:
			auth[3] = byte(0)
			binary.BigEndian.PutUint32(auth[4:], uint32(d.AuthHeader.SequenceNumber))
			copy(auth[8:], d.AuthHeader.Data)
		case BFDAuthTypeKeyedSHA1, BFDAuthTypeMeticulousKeyedSHA1:
			auth[3] = byte(0)
			binary.BigEndian.PutUint32(auth[4:], uint32(d.AuthHeader.SequenceNumber))
			copy(auth[8:], d.AuthHeader.Data)
		}
	}

	return nil
}

// CanDecode returns a set of layers that BFD objects can decode.
// As BFD objects can only decide the BFD layer, we can return just that layer.
// Apparently a single layer type implements LayerClass.
func (d *BFD) CanDecode() gopacket.LayerClass {
	return LayerTypeBFD
}

// NextLayerType specifies the next layer that GoPacket should attempt to
// analyse after this (BFD) layer. As BFD packets do not contain any payload
// bytes, there are no further layers to analyse.
func (d *BFD) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypeZero
}

// Payload returns an empty byte slice as BFD packets do not carry a payload
func (d *BFD) Payload() []byte {
	return nil
}

// bool2uint8 converts a bool to uint8
func bool2uint8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
