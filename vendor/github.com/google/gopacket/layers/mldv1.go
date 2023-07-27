// Copyright 2018 GoPacket Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net"
	"time"

	"github.com/google/gopacket"
)

// MLDv1Message represents the common structure of all MLDv1 messages
type MLDv1Message struct {
	BaseLayer
	// 3.4. Maximum Response Delay
	MaximumResponseDelay time.Duration
	// 3.6. Multicast Address
	// Zero in general query
	// Specific IPv6 multicast address otherwise
	MulticastAddress net.IP
}

// DecodeFromBytes decodes the given bytes into this layer.
func (m *MLDv1Message) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 20 {
		df.SetTruncated()
		return errors.New("ICMP layer less than 20 bytes for Multicast Listener Query Message V1")
	}

	m.MaximumResponseDelay = time.Duration(binary.BigEndian.Uint16(data[0:2])) * time.Millisecond
	// data[2:4] is reserved and not used in mldv1
	m.MulticastAddress = data[4:20]

	return nil
}

// NextLayerType returns the layer type contained by this DecodingLayer.
func (*MLDv1Message) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypeZero
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (m *MLDv1Message) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	buf, err := b.PrependBytes(20)
	if err != nil {
		return err
	}

	if m.MaximumResponseDelay < 0 {
		return errors.New("maximum response delay must not be negative")
	}
	dms := m.MaximumResponseDelay / time.Millisecond
	if dms > math.MaxUint16 {
		return fmt.Errorf("maximum response delay %dms is more than the allowed 65535ms", dms)
	}
	binary.BigEndian.PutUint16(buf[0:2], uint16(dms))

	copy(buf[2:4], []byte{0x0, 0x0})

	ma16 := m.MulticastAddress.To16()
	if ma16 == nil {
		return fmt.Errorf("invalid multicast address '%s'", m.MulticastAddress)
	}
	copy(buf[4:20], ma16)

	return nil
}

// Sums this layer up nicely formatted
func (m *MLDv1Message) String() string {
	return fmt.Sprintf(
		"Maximum Response Delay: %dms, Multicast Address: %s",
		m.MaximumResponseDelay/time.Millisecond,
		m.MulticastAddress)
}

// MLDv1MulticastListenerQueryMessage are sent by the router to determine
// whether there are multicast listeners on the link.
// https://tools.ietf.org/html/rfc2710 Page 5
type MLDv1MulticastListenerQueryMessage struct {
	MLDv1Message
}

// DecodeFromBytes decodes the given bytes into this layer.
func (m *MLDv1MulticastListenerQueryMessage) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	err := m.MLDv1Message.DecodeFromBytes(data, df)
	if err != nil {
		return err
	}

	if len(data) > 20 {
		m.Payload = data[20:]
	}

	return nil
}

// LayerType returns LayerTypeMLDv1MulticastListenerQuery.
func (*MLDv1MulticastListenerQueryMessage) LayerType() gopacket.LayerType {
	return LayerTypeMLDv1MulticastListenerQuery
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (*MLDv1MulticastListenerQueryMessage) CanDecode() gopacket.LayerClass {
	return LayerTypeMLDv1MulticastListenerQuery
}

// IsGeneralQuery is true when this is a general query.
// In a Query message, the Multicast Address field is set to zero when
// sending a General Query.
// https://tools.ietf.org/html/rfc2710#section-3.6
func (m *MLDv1MulticastListenerQueryMessage) IsGeneralQuery() bool {
	return net.IPv6zero.Equal(m.MulticastAddress)
}

// IsSpecificQuery is true when this is not a general query.
// In a Query message, the Multicast Address field is set to a specific
// IPv6 multicast address when sending a Multicast-Address-Specific Query.
// https://tools.ietf.org/html/rfc2710#section-3.6
func (m *MLDv1MulticastListenerQueryMessage) IsSpecificQuery() bool {
	return !m.IsGeneralQuery()
}

// MLDv1MulticastListenerReportMessage is sent by a client listening on
// a specific multicast address to indicate that it is (still) listening
// on the specific multicast address.
// https://tools.ietf.org/html/rfc2710 Page 6
type MLDv1MulticastListenerReportMessage struct {
	MLDv1Message
}

// LayerType returns LayerTypeMLDv1MulticastListenerReport.
func (*MLDv1MulticastListenerReportMessage) LayerType() gopacket.LayerType {
	return LayerTypeMLDv1MulticastListenerReport
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (*MLDv1MulticastListenerReportMessage) CanDecode() gopacket.LayerClass {
	return LayerTypeMLDv1MulticastListenerReport
}

// MLDv1MulticastListenerDoneMessage should be sent by a client when it ceases
// to listen to a multicast address on an interface.
// https://tools.ietf.org/html/rfc2710 Page 7
type MLDv1MulticastListenerDoneMessage struct {
	MLDv1Message
}

// LayerType returns LayerTypeMLDv1MulticastListenerDone.
func (*MLDv1MulticastListenerDoneMessage) LayerType() gopacket.LayerType {
	return LayerTypeMLDv1MulticastListenerDone
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (*MLDv1MulticastListenerDoneMessage) CanDecode() gopacket.LayerClass {
	return LayerTypeMLDv1MulticastListenerDone
}

func decodeMLDv1MulticastListenerReport(data []byte, p gopacket.PacketBuilder) error {
	m := &MLDv1MulticastListenerReportMessage{}
	return decodingLayerDecoder(m, data, p)
}

func decodeMLDv1MulticastListenerQuery(data []byte, p gopacket.PacketBuilder) error {
	m := &MLDv1MulticastListenerQueryMessage{}
	return decodingLayerDecoder(m, data, p)
}

func decodeMLDv1MulticastListenerDone(data []byte, p gopacket.PacketBuilder) error {
	m := &MLDv1MulticastListenerDoneMessage{}
	return decodingLayerDecoder(m, data, p)
}
