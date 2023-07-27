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

const (
	// S Flag bit is 1
	mldv2STrue uint8 = 0x8

	// S Flag value mask
	//   mldv2STrue & mldv2SMask == mldv2STrue  // true
	//          0x1 & mldv2SMask == mldv2STrue  // true
	//          0x0 & mldv2SMask == mldv2STrue  // false
	mldv2SMask uint8 = 0x8

	// QRV value mask
	mldv2QRVMask uint8 = 0x7
)

// MLDv2MulticastListenerQueryMessage are sent by multicast routers to query the
// multicast listening state of neighboring interfaces.
// https://tools.ietf.org/html/rfc3810#section-5.1
//
// Some information, like Maximum Response Code and Multicast Address are in the
// previous layer LayerTypeMLDv1MulticastListenerQuery
type MLDv2MulticastListenerQueryMessage struct {
	BaseLayer
	// 5.1.3. Maximum Response Delay COde
	MaximumResponseCode uint16
	// 5.1.5. Multicast Address
	// Zero in general query
	// Specific IPv6 multicast address otherwise
	MulticastAddress net.IP
	// 5.1.7. S Flag (Suppress Router-Side Processing)
	SuppressRoutersideProcessing bool
	// 5.1.8. QRV (Querier's Robustness Variable)
	QueriersRobustnessVariable uint8
	// 5.1.9. QQIC (Querier's Query Interval Code)
	QueriersQueryIntervalCode uint8
	// 5.1.10. Number of Sources (N)
	NumberOfSources uint16
	// 5.1.11 Source Address [i]
	SourceAddresses []net.IP
}

// DecodeFromBytes decodes the given bytes into this layer.
func (m *MLDv2MulticastListenerQueryMessage) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 24 {
		df.SetTruncated()
		return errors.New("ICMP layer less than 24 bytes for Multicast Listener Query Message V2")
	}

	m.MaximumResponseCode = binary.BigEndian.Uint16(data[0:2])
	// ignore data[2:4] as per https://tools.ietf.org/html/rfc3810#section-5.1.4
	m.MulticastAddress = data[4:20]
	m.SuppressRoutersideProcessing = (data[20] & mldv2SMask) == mldv2STrue
	m.QueriersRobustnessVariable = data[20] & mldv2QRVMask
	m.QueriersQueryIntervalCode = data[21]

	m.NumberOfSources = binary.BigEndian.Uint16(data[22:24])

	var end int
	for i := uint16(0); i < m.NumberOfSources; i++ {
		begin := 24 + (int(i) * 16)
		end = begin + 16

		if end > len(data) {
			df.SetTruncated()
			return fmt.Errorf("ICMP layer less than %d bytes for Multicast Listener Query Message V2", end)
		}

		m.SourceAddresses = append(m.SourceAddresses, data[begin:end])
	}

	return nil
}

// NextLayerType returns the layer type contained by this DecodingLayer.
func (*MLDv2MulticastListenerQueryMessage) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypeZero
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (m *MLDv2MulticastListenerQueryMessage) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	if err := m.serializeSourceAddressesTo(b, opts); err != nil {
		return err
	}

	buf, err := b.PrependBytes(24)
	if err != nil {
		return err
	}

	binary.BigEndian.PutUint16(buf[0:2], m.MaximumResponseCode)
	copy(buf[2:4], []byte{0x00, 0x00}) // set reserved bytes to zero

	ma16 := m.MulticastAddress.To16()
	if ma16 == nil {
		return fmt.Errorf("invalid MulticastAddress '%s'", m.MulticastAddress)
	}
	copy(buf[4:20], ma16)

	byte20 := m.QueriersRobustnessVariable & mldv2QRVMask
	if m.SuppressRoutersideProcessing {
		byte20 |= mldv2STrue
	} else {
		byte20 &= ^mldv2STrue // the complement of mldv2STrue
	}
	byte20 &= 0x0F // set reserved bits to zero
	buf[20] = byte20

	binary.BigEndian.PutUint16(buf[22:24], m.NumberOfSources)
	buf[21] = m.QueriersQueryIntervalCode

	return nil
}

// writes each source address to the buffer preserving the order
func (m *MLDv2MulticastListenerQueryMessage) serializeSourceAddressesTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	numberOfSourceAddresses := len(m.SourceAddresses)
	if numberOfSourceAddresses > math.MaxUint16 {
		return fmt.Errorf(
			"there are more than %d source addresses, but 65535 is the maximum number of supported addresses",
			numberOfSourceAddresses)
	}

	if opts.FixLengths {
		m.NumberOfSources = uint16(numberOfSourceAddresses)
	}

	lastSAIdx := numberOfSourceAddresses - 1
	for k := range m.SourceAddresses {
		i := lastSAIdx - k // reverse order

		buf, err := b.PrependBytes(16)
		if err != nil {
			return err
		}

		sa16 := m.SourceAddresses[i].To16()
		if sa16 == nil {
			return fmt.Errorf("invalid source address [%d] '%s'", i, m.SourceAddresses[i])
		}
		copy(buf[0:16], sa16)
	}

	return nil
}

// String sums this layer up nicely formatted
func (m *MLDv2MulticastListenerQueryMessage) String() string {
	return fmt.Sprintf(
		"Maximum Response Code: %#x (%dms), Multicast Address: %s, Suppress Routerside Processing: %t, QRV: %#x, QQIC: %#x (%ds), Number of Source Address: %d (actual: %d), Source Addresses: %s",
		m.MaximumResponseCode,
		m.MaximumResponseDelay(),
		m.MulticastAddress,
		m.SuppressRoutersideProcessing,
		m.QueriersRobustnessVariable,
		m.QueriersQueryIntervalCode,
		m.QQI()/time.Second,
		m.NumberOfSources,
		len(m.SourceAddresses),
		m.SourceAddresses)
}

// LayerType returns LayerTypeMLDv2MulticastListenerQuery.
func (*MLDv2MulticastListenerQueryMessage) LayerType() gopacket.LayerType {
	return LayerTypeMLDv2MulticastListenerQuery
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (*MLDv2MulticastListenerQueryMessage) CanDecode() gopacket.LayerClass {
	return LayerTypeMLDv2MulticastListenerQuery
}

// QQI calculates the Querier's Query Interval based on the QQIC
// according to https://tools.ietf.org/html/rfc3810#section-5.1.9
func (m *MLDv2MulticastListenerQueryMessage) QQI() time.Duration {
	data := m.QueriersQueryIntervalCode
	if data < 128 {
		return time.Second * time.Duration(data)
	}

	exp := uint16(data) & 0x70 >> 4
	mant := uint16(data) & 0x0F
	return time.Second * time.Duration(mant|0x1000<<(exp+3))
}

// SetQQI calculates and updates the Querier's Query Interval Code (QQIC)
// according to https://tools.ietf.org/html/rfc3810#section-5.1.9
func (m *MLDv2MulticastListenerQueryMessage) SetQQI(d time.Duration) error {
	if d < 0 {
		m.QueriersQueryIntervalCode = 0
		return errors.New("QQI duration is negative")
	}

	if d == 0 {
		m.QueriersQueryIntervalCode = 0
		return nil
	}

	dms := d / time.Second
	if dms < 128 {
		m.QueriersQueryIntervalCode = uint8(dms)
	}

	if dms > 31744 { // mant=0xF, exp=0x7
		m.QueriersQueryIntervalCode = 0xFF
		return fmt.Errorf("QQI duration %ds is, maximum allowed is 31744s", dms)
	}

	value := uint16(dms) // ok, because 31744 < math.MaxUint16
	exp := uint8(7)
	for mask := uint16(0x4000); exp > 0; exp-- {
		if mask&value != 0 {
			break
		}

		mask >>= 1
	}

	mant := uint8(0x000F & (value >> (exp + 3)))
	sig := uint8(0x10)
	m.QueriersQueryIntervalCode = sig | exp<<4 | mant

	return nil
}

// MaximumResponseDelay returns the Maximum Response Delay based on the
// Maximum Response Code according to
// https://tools.ietf.org/html/rfc3810#section-5.1.3
func (m *MLDv2MulticastListenerQueryMessage) MaximumResponseDelay() time.Duration {
	if m.MaximumResponseCode < 0x8000 {
		return time.Duration(m.MaximumResponseCode)
	}

	exp := m.MaximumResponseCode & 0x7000 >> 12
	mant := m.MaximumResponseCode & 0x0FFF

	return time.Millisecond * time.Duration(mant|0x1000<<(exp+3))
}

// SetMLDv2MaximumResponseDelay updates the Maximum Response Code according to
// https://tools.ietf.org/html/rfc3810#section-5.1.3
func (m *MLDv2MulticastListenerQueryMessage) SetMLDv2MaximumResponseDelay(d time.Duration) error {
	if d == 0 {
		m.MaximumResponseCode = 0
		return nil
	}

	if d < 0 {
		return errors.New("maximum response delay must not be negative")
	}

	dms := d / time.Millisecond

	if dms < 32768 {
		m.MaximumResponseCode = uint16(dms)
	}

	if dms > 4193280 { // mant=0xFFF, exp=0x7
		return fmt.Errorf("maximum response delay %dms is bigger the than maximum of 4193280ms", dms)
	}

	value := uint32(dms) // ok, because 4193280 < math.MaxUint32
	exp := uint8(7)
	for mask := uint32(0x40000000); exp > 0; exp-- {
		if mask&value != 0 {
			break
		}

		mask >>= 1
	}

	mant := uint16(0x00000FFF & (value >> (exp + 3)))
	sig := uint16(0x1000)
	m.MaximumResponseCode = sig | uint16(exp)<<12 | mant
	return nil
}

// MLDv2MulticastListenerReportMessage is sent by an IP node to report the
// current multicast listening state, or changes therein.
// https://tools.ietf.org/html/rfc3810#section-5.2
type MLDv2MulticastListenerReportMessage struct {
	BaseLayer
	// 5.2.3. Nr of Mcast Address Records
	NumberOfMulticastAddressRecords uint16
	// 5.2.4. Multicast Address Record [i]
	MulticastAddressRecords []MLDv2MulticastAddressRecord
}

// DecodeFromBytes decodes the given bytes into this layer.
func (m *MLDv2MulticastListenerReportMessage) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 4 {
		df.SetTruncated()
		return errors.New("ICMP layer less than 4 bytes for Multicast Listener Report Message V2")
	}

	// ignore data[0:2] as per RFC
	// https://tools.ietf.org/html/rfc3810#section-5.2.1
	m.NumberOfMulticastAddressRecords = binary.BigEndian.Uint16(data[2:4])

	begin := 4
	for i := uint16(0); i < m.NumberOfMulticastAddressRecords; i++ {
		mar := MLDv2MulticastAddressRecord{}
		read, err := mar.decode(data[begin:], df)
		if err != nil {
			return err
		}

		m.MulticastAddressRecords = append(m.MulticastAddressRecords, mar)

		begin += read
	}

	return nil
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (m *MLDv2MulticastListenerReportMessage) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	lastItemIdx := len(m.MulticastAddressRecords) - 1
	for k := range m.MulticastAddressRecords {
		i := lastItemIdx - k // reverse order

		err := m.MulticastAddressRecords[i].serializeTo(b, opts)
		if err != nil {
			return err
		}
	}

	if opts.FixLengths {
		numberOfMAR := len(m.MulticastAddressRecords)
		if numberOfMAR > math.MaxUint16 {
			return fmt.Errorf(
				"%d multicast address records added, but the maximum is 65535",
				numberOfMAR)
		}

		m.NumberOfMulticastAddressRecords = uint16(numberOfMAR)
	}

	buf, err := b.PrependBytes(4)
	if err != nil {
		return err
	}

	copy(buf[0:2], []byte{0x0, 0x0})
	binary.BigEndian.PutUint16(buf[2:4], m.NumberOfMulticastAddressRecords)
	return nil
}

// Sums this layer up nicely formatted
func (m *MLDv2MulticastListenerReportMessage) String() string {
	return fmt.Sprintf(
		"Number of Mcast Addr Records: %d (actual %d), Multicast Address Records: %+v",
		m.NumberOfMulticastAddressRecords,
		len(m.MulticastAddressRecords),
		m.MulticastAddressRecords)
}

// LayerType returns LayerTypeMLDv2MulticastListenerQuery.
func (*MLDv2MulticastListenerReportMessage) LayerType() gopacket.LayerType {
	return LayerTypeMLDv2MulticastListenerReport
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (*MLDv2MulticastListenerReportMessage) CanDecode() gopacket.LayerClass {
	return LayerTypeMLDv2MulticastListenerReport
}

// NextLayerType returns the layer type contained by this DecodingLayer.
func (*MLDv2MulticastListenerReportMessage) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

// MLDv2MulticastAddressRecordType holds the type of a
// Multicast Address Record, according to
// https://tools.ietf.org/html/rfc3810#section-5.2.5 and
// https://tools.ietf.org/html/rfc3810#section-5.2.12
type MLDv2MulticastAddressRecordType uint8

const (
	// MLDv2MulticastAddressRecordTypeModeIsIncluded stands for
	// MODE_IS_INCLUDE - indicates that the interface has a filter
	// mode of INCLUDE for the specified multicast address.
	MLDv2MulticastAddressRecordTypeModeIsIncluded MLDv2MulticastAddressRecordType = 1
	// MLDv2MulticastAddressRecordTypeModeIsExcluded stands for
	// MODE_IS_EXCLUDE - indicates that the interface has a filter
	// mode of EXCLUDE for the specified multicast address.
	MLDv2MulticastAddressRecordTypeModeIsExcluded MLDv2MulticastAddressRecordType = 2
	// MLDv2MulticastAddressRecordTypeChangeToIncludeMode stands for
	// CHANGE_TO_INCLUDE_MODE - indicates that the interface has
	// changed to INCLUDE filter mode for the specified multicast
	// address.
	MLDv2MulticastAddressRecordTypeChangeToIncludeMode MLDv2MulticastAddressRecordType = 3
	// MLDv2MulticastAddressRecordTypeChangeToExcludeMode stands for
	// CHANGE_TO_EXCLUDE_MODE - indicates that the interface has
	// changed to EXCLUDE filter mode for the specified multicast
	// address
	MLDv2MulticastAddressRecordTypeChangeToExcludeMode MLDv2MulticastAddressRecordType = 4
	// MLDv2MulticastAddressRecordTypeAllowNewSources stands for
	// ALLOW_NEW_SOURCES - indicates that the Source Address [i]
	// fields in this Multicast Address Record contain a list of
	// the additional sources that the node wishes to listen to,
	// for packets sent to the specified multicast address.
	MLDv2MulticastAddressRecordTypeAllowNewSources MLDv2MulticastAddressRecordType = 5
	// MLDv2MulticastAddressRecordTypeBlockOldSources stands for
	// BLOCK_OLD_SOURCES - indicates that the Source Address [i]
	// fields in this Multicast Address Record contain a list of
	// the sources that the node no longer wishes to listen to,
	// for packets sent to the specified multicast address.
	MLDv2MulticastAddressRecordTypeBlockOldSources MLDv2MulticastAddressRecordType = 6
)

// Human readable record types
// Naming follows https://tools.ietf.org/html/rfc3810#section-5.2.12
func (m MLDv2MulticastAddressRecordType) String() string {
	switch m {
	case MLDv2MulticastAddressRecordTypeModeIsIncluded:
		return "MODE_IS_INCLUDE"
	case MLDv2MulticastAddressRecordTypeModeIsExcluded:
		return "MODE_IS_EXCLUDE"
	case MLDv2MulticastAddressRecordTypeChangeToIncludeMode:
		return "CHANGE_TO_INCLUDE_MODE"
	case MLDv2MulticastAddressRecordTypeChangeToExcludeMode:
		return "CHANGE_TO_EXCLUDE_MODE"
	case MLDv2MulticastAddressRecordTypeAllowNewSources:
		return "ALLOW_NEW_SOURCES"
	case MLDv2MulticastAddressRecordTypeBlockOldSources:
		return "BLOCK_OLD_SOURCES"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", m)
	}
}

// MLDv2MulticastAddressRecord contains information on the sender listening to a
// single multicast address on the interface the report is sent.
// https://tools.ietf.org/html/rfc3810#section-5.2.4
type MLDv2MulticastAddressRecord struct {
	// 5.2.5. Record Type
	RecordType MLDv2MulticastAddressRecordType
	// 5.2.6. Auxiliary Data Length (number of 32-bit words)
	AuxDataLen uint8
	// 5.2.7. Number Of Sources (N)
	N uint16
	// 5.2.8. Multicast Address
	MulticastAddress net.IP
	// 5.2.9 Source Address [i]
	SourceAddresses []net.IP
	// 5.2.10 Auxiliary Data
	AuxiliaryData []byte
}

// decodes a multicast address record from bytes
func (m *MLDv2MulticastAddressRecord) decode(data []byte, df gopacket.DecodeFeedback) (int, error) {
	if len(data) < 20 {
		df.SetTruncated()
		return 0, errors.New(
			"Multicast Listener Report Message V2 layer less than 4 bytes for Multicast Address Record")
	}

	m.RecordType = MLDv2MulticastAddressRecordType(data[0])
	m.AuxDataLen = data[1]
	m.N = binary.BigEndian.Uint16(data[2:4])
	m.MulticastAddress = data[4:20]

	for i := uint16(0); i < m.N; i++ {
		begin := 20 + (int(i) * 16)
		end := begin + 16

		if len(data) < end {
			df.SetTruncated()
			return begin, fmt.Errorf(
				"Multicast Listener Report Message V2 layer less than %d bytes for Multicast Address Record", end)
		}

		m.SourceAddresses = append(m.SourceAddresses, data[begin:end])
	}

	expectedLengthWithouAuxData := 20 + (int(m.N) * 16)
	expectedTotalLength := (int(m.AuxDataLen) * 4) + expectedLengthWithouAuxData // *4 because AuxDataLen are 32bit words
	if len(data) < expectedTotalLength {
		return expectedLengthWithouAuxData, fmt.Errorf(
			"Multicast Listener Report Message V2 layer less than %d bytes for Multicast Address Record",
			expectedLengthWithouAuxData)
	}

	m.AuxiliaryData = data[expectedLengthWithouAuxData:expectedTotalLength]

	return expectedTotalLength, nil
}

// String sums this layer up nicely formatted
func (m *MLDv2MulticastAddressRecord) String() string {
	return fmt.Sprintf(
		"RecordType: %d (%s), AuxDataLen: %d [32-bit words], N: %d, Multicast Address: %s, SourceAddresses: %s, Auxiliary Data: %#x",
		m.RecordType,
		m.RecordType.String(),
		m.AuxDataLen,
		m.N,
		m.MulticastAddress.To16(),
		m.SourceAddresses,
		m.AuxiliaryData)
}

// serializes a multicast address record
func (m *MLDv2MulticastAddressRecord) serializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	if err := m.serializeAuxiliaryDataTo(b, opts); err != nil {
		return err
	}

	if err := m.serializeSourceAddressesTo(b, opts); err != nil {
		return err
	}

	buf, err := b.PrependBytes(20)
	if err != nil {
		return err
	}

	buf[0] = uint8(m.RecordType)
	buf[1] = m.AuxDataLen
	binary.BigEndian.PutUint16(buf[2:4], m.N)

	ma16 := m.MulticastAddress.To16()
	if ma16 == nil {
		return fmt.Errorf("invalid multicast address '%s'", m.MulticastAddress)
	}
	copy(buf[4:20], ma16)

	return nil
}

// serializes the auxiliary data of a multicast address record
func (m *MLDv2MulticastAddressRecord) serializeAuxiliaryDataTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	if remainder := len(m.AuxiliaryData) % 4; remainder != 0 {
		zeroWord := []byte{0x0, 0x0, 0x0, 0x0}
		m.AuxiliaryData = append(m.AuxiliaryData, zeroWord[:remainder]...)
	}

	if opts.FixLengths {
		auxDataLen := len(m.AuxiliaryData) / 4

		if auxDataLen > math.MaxUint8 {
			return fmt.Errorf("auxilary data is %d 32-bit words, but the maximum is 255 32-bit words", auxDataLen)
		}

		m.AuxDataLen = uint8(auxDataLen)
	}

	buf, err := b.PrependBytes(len(m.AuxiliaryData))
	if err != nil {
		return err
	}

	copy(buf, m.AuxiliaryData)
	return nil
}

// serializes the source addresses of a multicast address record preserving the order
func (m *MLDv2MulticastAddressRecord) serializeSourceAddressesTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	if opts.FixLengths {
		numberOfSourceAddresses := len(m.SourceAddresses)

		if numberOfSourceAddresses > math.MaxUint16 {
			return fmt.Errorf(
				"%d source addresses added, but the maximum is 65535",
				numberOfSourceAddresses)
		}

		m.N = uint16(numberOfSourceAddresses)
	}

	lastItemIdx := len(m.SourceAddresses) - 1
	for k := range m.SourceAddresses {
		i := lastItemIdx - k // reverse order

		buf, err := b.PrependBytes(16)
		if err != nil {
			return err
		}

		sa16 := m.SourceAddresses[i].To16()
		if sa16 == nil {
			return fmt.Errorf("invalid source address [%d] '%s'", i, m.SourceAddresses[i])
		}
		copy(buf, sa16)
	}

	return nil
}

func decodeMLDv2MulticastListenerReport(data []byte, p gopacket.PacketBuilder) error {
	m := &MLDv2MulticastListenerReportMessage{}
	return decodingLayerDecoder(m, data, p)
}

func decodeMLDv2MulticastListenerQuery(data []byte, p gopacket.PacketBuilder) error {
	m := &MLDv2MulticastListenerQueryMessage{}
	return decodingLayerDecoder(m, data, p)
}
