// Copyright 2012 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"

	"github.com/google/gopacket"
)

// SCTP contains information on the top level of an SCTP packet.
type SCTP struct {
	BaseLayer
	SrcPort, DstPort SCTPPort
	VerificationTag  uint32
	Checksum         uint32
	sPort, dPort     []byte
}

// LayerType returns gopacket.LayerTypeSCTP
func (s *SCTP) LayerType() gopacket.LayerType { return LayerTypeSCTP }

func decodeSCTP(data []byte, p gopacket.PacketBuilder) error {
	sctp := &SCTP{}
	err := sctp.DecodeFromBytes(data, p)
	p.AddLayer(sctp)
	p.SetTransportLayer(sctp)
	if err != nil {
		return err
	}
	return p.NextDecoder(sctpChunkTypePrefixDecoder)
}

var sctpChunkTypePrefixDecoder = gopacket.DecodeFunc(decodeWithSCTPChunkTypePrefix)

// TransportFlow returns a flow based on the source and destination SCTP port.
func (s *SCTP) TransportFlow() gopacket.Flow {
	return gopacket.NewFlow(EndpointSCTPPort, s.sPort, s.dPort)
}

func decodeWithSCTPChunkTypePrefix(data []byte, p gopacket.PacketBuilder) error {
	chunkType := SCTPChunkType(data[0])
	return chunkType.Decode(data, p)
}

// SerializeTo is for gopacket.SerializableLayer.
func (s SCTP) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	bytes, err := b.PrependBytes(12)
	if err != nil {
		return err
	}
	binary.BigEndian.PutUint16(bytes[0:2], uint16(s.SrcPort))
	binary.BigEndian.PutUint16(bytes[2:4], uint16(s.DstPort))
	binary.BigEndian.PutUint32(bytes[4:8], s.VerificationTag)
	if opts.ComputeChecksums {
		// Note:  MakeTable(Castagnoli) actually only creates the table once, then
		// passes back a singleton on every other call, so this shouldn't cause
		// excessive memory allocation.
		binary.LittleEndian.PutUint32(bytes[8:12], crc32.Checksum(b.Bytes(), crc32.MakeTable(crc32.Castagnoli)))
	}
	return nil
}

func (sctp *SCTP) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 12 {
		return errors.New("Invalid SCTP common header length")
	}
	sctp.SrcPort = SCTPPort(binary.BigEndian.Uint16(data[:2]))
	sctp.sPort = data[:2]
	sctp.DstPort = SCTPPort(binary.BigEndian.Uint16(data[2:4]))
	sctp.dPort = data[2:4]
	sctp.VerificationTag = binary.BigEndian.Uint32(data[4:8])
	sctp.Checksum = binary.BigEndian.Uint32(data[8:12])
	sctp.BaseLayer = BaseLayer{data[:12], data[12:]}

	return nil
}

func (t *SCTP) CanDecode() gopacket.LayerClass {
	return LayerTypeSCTP
}

func (t *SCTP) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

// SCTPChunk contains the common fields in all SCTP chunks.
type SCTPChunk struct {
	BaseLayer
	Type   SCTPChunkType
	Flags  uint8
	Length uint16
	// ActualLength is the total length of an SCTP chunk, including padding.
	// SCTP chunks start and end on 4-byte boundaries.  So if a chunk has a length
	// of 18, it means that it has data up to and including byte 18, then padding
	// up to the next 4-byte boundary, 20.  In this case, Length would be 18, and
	// ActualLength would be 20.
	ActualLength int
}

func roundUpToNearest4(i int) int {
	if i%4 == 0 {
		return i
	}
	return i + 4 - (i % 4)
}

func decodeSCTPChunk(data []byte) (SCTPChunk, error) {
	length := binary.BigEndian.Uint16(data[2:4])
	if length < 4 {
		return SCTPChunk{}, errors.New("invalid SCTP chunk length")
	}
	actual := roundUpToNearest4(int(length))
	ct := SCTPChunkType(data[0])

	// For SCTP Data, use a separate layer for the payload
	delta := 0
	if ct == SCTPChunkTypeData {
		delta = int(actual) - int(length)
		actual = 16
	}

	return SCTPChunk{
		Type:         ct,
		Flags:        data[1],
		Length:       length,
		ActualLength: actual,
		BaseLayer:    BaseLayer{data[:actual], data[actual : len(data)-delta]},
	}, nil
}

// SCTPParameter is a TLV parameter inside a SCTPChunk.
type SCTPParameter struct {
	Type         uint16
	Length       uint16
	ActualLength int
	Value        []byte
}

func decodeSCTPParameter(data []byte) SCTPParameter {
	length := binary.BigEndian.Uint16(data[2:4])
	return SCTPParameter{
		Type:         binary.BigEndian.Uint16(data[0:2]),
		Length:       length,
		Value:        data[4:length],
		ActualLength: roundUpToNearest4(int(length)),
	}
}

func (p SCTPParameter) Bytes() []byte {
	length := 4 + len(p.Value)
	data := make([]byte, roundUpToNearest4(length))
	binary.BigEndian.PutUint16(data[0:2], p.Type)
	binary.BigEndian.PutUint16(data[2:4], uint16(length))
	copy(data[4:], p.Value)
	return data
}

// SCTPUnknownChunkType is the layer type returned when we don't recognize the
// chunk type.  Since there's a length in a known location, we can skip over
// it even if we don't know what it is, and continue parsing the rest of the
// chunks.  This chunk is stored as an ErrorLayer in the packet.
type SCTPUnknownChunkType struct {
	SCTPChunk
	bytes []byte
}

func decodeSCTPChunkTypeUnknown(data []byte, p gopacket.PacketBuilder) error {
	chunk, err := decodeSCTPChunk(data)
	if err != nil {
		return err
	}
	sc := &SCTPUnknownChunkType{SCTPChunk: chunk}
	sc.bytes = data[:sc.ActualLength]
	p.AddLayer(sc)
	p.SetErrorLayer(sc)
	return p.NextDecoder(gopacket.DecodeFunc(decodeWithSCTPChunkTypePrefix))
}

// SerializeTo is for gopacket.SerializableLayer.
func (s SCTPUnknownChunkType) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	bytes, err := b.PrependBytes(s.ActualLength)
	if err != nil {
		return err
	}
	copy(bytes, s.bytes)
	return nil
}

// LayerType returns gopacket.LayerTypeSCTPUnknownChunkType.
func (s *SCTPUnknownChunkType) LayerType() gopacket.LayerType { return LayerTypeSCTPUnknownChunkType }

// Payload returns all bytes in this header, including the decoded Type, Length,
// and Flags.
func (s *SCTPUnknownChunkType) Payload() []byte { return s.bytes }

// Error implements ErrorLayer.
func (s *SCTPUnknownChunkType) Error() error {
	return fmt.Errorf("No decode method available for SCTP chunk type %s", s.Type)
}

// SCTPData is the SCTP Data chunk layer.
type SCTPData struct {
	SCTPChunk
	Unordered, BeginFragment, EndFragment bool
	TSN                                   uint32
	StreamId                              uint16
	StreamSequence                        uint16
	PayloadProtocol                       SCTPPayloadProtocol
}

// LayerType returns gopacket.LayerTypeSCTPData.
func (s *SCTPData) LayerType() gopacket.LayerType { return LayerTypeSCTPData }

// SCTPPayloadProtocol represents a payload protocol
type SCTPPayloadProtocol uint32

// SCTPPayloadProtocol constonts from http://www.iana.org/assignments/sctp-parameters/sctp-parameters.xhtml
const (
	SCTPProtocolReserved  SCTPPayloadProtocol = 0
	SCTPPayloadUIA                            = 1
	SCTPPayloadM2UA                           = 2
	SCTPPayloadM3UA                           = 3
	SCTPPayloadSUA                            = 4
	SCTPPayloadM2PA                           = 5
	SCTPPayloadV5UA                           = 6
	SCTPPayloadH248                           = 7
	SCTPPayloadBICC                           = 8
	SCTPPayloadTALI                           = 9
	SCTPPayloadDUA                            = 10
	SCTPPayloadASAP                           = 11
	SCTPPayloadENRP                           = 12
	SCTPPayloadH323                           = 13
	SCTPPayloadQIPC                           = 14
	SCTPPayloadSIMCO                          = 15
	SCTPPayloadDDPSegment                     = 16
	SCTPPayloadDDPStream                      = 17
	SCTPPayloadS1AP                           = 18
)

func (p SCTPPayloadProtocol) String() string {
	switch p {
	case SCTPProtocolReserved:
		return "Reserved"
	case SCTPPayloadUIA:
		return "UIA"
	case SCTPPayloadM2UA:
		return "M2UA"
	case SCTPPayloadM3UA:
		return "M3UA"
	case SCTPPayloadSUA:
		return "SUA"
	case SCTPPayloadM2PA:
		return "M2PA"
	case SCTPPayloadV5UA:
		return "V5UA"
	case SCTPPayloadH248:
		return "H.248"
	case SCTPPayloadBICC:
		return "BICC"
	case SCTPPayloadTALI:
		return "TALI"
	case SCTPPayloadDUA:
		return "DUA"
	case SCTPPayloadASAP:
		return "ASAP"
	case SCTPPayloadENRP:
		return "ENRP"
	case SCTPPayloadH323:
		return "H.323"
	case SCTPPayloadQIPC:
		return "QIPC"
	case SCTPPayloadSIMCO:
		return "SIMCO"
	case SCTPPayloadDDPSegment:
		return "DDPSegment"
	case SCTPPayloadDDPStream:
		return "DDPStream"
	case SCTPPayloadS1AP:
		return "S1AP"
	}
	return fmt.Sprintf("Unknown(%d)", p)
}

func decodeSCTPData(data []byte, p gopacket.PacketBuilder) error {
	chunk, err := decodeSCTPChunk(data)
	if err != nil {
		return err
	}
	sc := &SCTPData{
		SCTPChunk:       chunk,
		Unordered:       data[1]&0x4 != 0,
		BeginFragment:   data[1]&0x2 != 0,
		EndFragment:     data[1]&0x1 != 0,
		TSN:             binary.BigEndian.Uint32(data[4:8]),
		StreamId:        binary.BigEndian.Uint16(data[8:10]),
		StreamSequence:  binary.BigEndian.Uint16(data[10:12]),
		PayloadProtocol: SCTPPayloadProtocol(binary.BigEndian.Uint32(data[12:16])),
	}
	// Length is the length in bytes of the data, INCLUDING the 16-byte header.
	p.AddLayer(sc)
	return p.NextDecoder(gopacket.LayerTypePayload)
}

// SerializeTo is for gopacket.SerializableLayer.
func (sc SCTPData) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	payload := b.Bytes()
	// Pad the payload to a 32 bit boundary
	if rem := len(payload) % 4; rem != 0 {
		b.AppendBytes(4 - rem)
	}
	length := 16
	bytes, err := b.PrependBytes(length)
	if err != nil {
		return err
	}
	bytes[0] = uint8(sc.Type)
	flags := uint8(0)
	if sc.Unordered {
		flags |= 0x4
	}
	if sc.BeginFragment {
		flags |= 0x2
	}
	if sc.EndFragment {
		flags |= 0x1
	}
	bytes[1] = flags
	binary.BigEndian.PutUint16(bytes[2:4], uint16(length+len(payload)))
	binary.BigEndian.PutUint32(bytes[4:8], sc.TSN)
	binary.BigEndian.PutUint16(bytes[8:10], sc.StreamId)
	binary.BigEndian.PutUint16(bytes[10:12], sc.StreamSequence)
	binary.BigEndian.PutUint32(bytes[12:16], uint32(sc.PayloadProtocol))
	return nil
}

// SCTPInitParameter is a parameter for an SCTP Init or InitAck packet.
type SCTPInitParameter SCTPParameter

// SCTPInit is used as the return value for both SCTPInit and SCTPInitAck
// messages.
type SCTPInit struct {
	SCTPChunk
	InitiateTag                     uint32
	AdvertisedReceiverWindowCredit  uint32
	OutboundStreams, InboundStreams uint16
	InitialTSN                      uint32
	Parameters                      []SCTPInitParameter
}

// LayerType returns either gopacket.LayerTypeSCTPInit or gopacket.LayerTypeSCTPInitAck.
func (sc *SCTPInit) LayerType() gopacket.LayerType {
	if sc.Type == SCTPChunkTypeInitAck {
		return LayerTypeSCTPInitAck
	}
	// sc.Type == SCTPChunkTypeInit
	return LayerTypeSCTPInit
}

func decodeSCTPInit(data []byte, p gopacket.PacketBuilder) error {
	chunk, err := decodeSCTPChunk(data)
	if err != nil {
		return err
	}
	sc := &SCTPInit{
		SCTPChunk:                      chunk,
		InitiateTag:                    binary.BigEndian.Uint32(data[4:8]),
		AdvertisedReceiverWindowCredit: binary.BigEndian.Uint32(data[8:12]),
		OutboundStreams:                binary.BigEndian.Uint16(data[12:14]),
		InboundStreams:                 binary.BigEndian.Uint16(data[14:16]),
		InitialTSN:                     binary.BigEndian.Uint32(data[16:20]),
	}
	paramData := data[20:sc.ActualLength]
	for len(paramData) > 0 {
		p := SCTPInitParameter(decodeSCTPParameter(paramData))
		paramData = paramData[p.ActualLength:]
		sc.Parameters = append(sc.Parameters, p)
	}
	p.AddLayer(sc)
	return p.NextDecoder(gopacket.DecodeFunc(decodeWithSCTPChunkTypePrefix))
}

// SerializeTo is for gopacket.SerializableLayer.
func (sc SCTPInit) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	var payload []byte
	for _, param := range sc.Parameters {
		payload = append(payload, SCTPParameter(param).Bytes()...)
	}
	length := 20 + len(payload)
	bytes, err := b.PrependBytes(roundUpToNearest4(length))
	if err != nil {
		return err
	}
	bytes[0] = uint8(sc.Type)
	bytes[1] = sc.Flags
	binary.BigEndian.PutUint16(bytes[2:4], uint16(length))
	binary.BigEndian.PutUint32(bytes[4:8], sc.InitiateTag)
	binary.BigEndian.PutUint32(bytes[8:12], sc.AdvertisedReceiverWindowCredit)
	binary.BigEndian.PutUint16(bytes[12:14], sc.OutboundStreams)
	binary.BigEndian.PutUint16(bytes[14:16], sc.InboundStreams)
	binary.BigEndian.PutUint32(bytes[16:20], sc.InitialTSN)
	copy(bytes[20:], payload)
	return nil
}

// SCTPSack is the SCTP Selective ACK chunk layer.
type SCTPSack struct {
	SCTPChunk
	CumulativeTSNAck               uint32
	AdvertisedReceiverWindowCredit uint32
	NumGapACKs, NumDuplicateTSNs   uint16
	GapACKs                        []uint16
	DuplicateTSNs                  []uint32
}

// LayerType return LayerTypeSCTPSack
func (sc *SCTPSack) LayerType() gopacket.LayerType {
	return LayerTypeSCTPSack
}

func decodeSCTPSack(data []byte, p gopacket.PacketBuilder) error {
	chunk, err := decodeSCTPChunk(data)
	if err != nil {
		return err
	}
	sc := &SCTPSack{
		SCTPChunk:                      chunk,
		CumulativeTSNAck:               binary.BigEndian.Uint32(data[4:8]),
		AdvertisedReceiverWindowCredit: binary.BigEndian.Uint32(data[8:12]),
		NumGapACKs:                     binary.BigEndian.Uint16(data[12:14]),
		NumDuplicateTSNs:               binary.BigEndian.Uint16(data[14:16]),
	}
	// We maximize gapAcks and dupTSNs here so we're not allocating tons
	// of memory based on a user-controlable field.  Our maximums are not exact,
	// but should give us sane defaults... we'll still hit slice boundaries and
	// fail if the user-supplied values are too high (in the for loops below), but
	// the amount of memory we'll have allocated because of that should be small
	// (< sc.ActualLength)
	gapAcks := sc.SCTPChunk.ActualLength / 2
	dupTSNs := (sc.SCTPChunk.ActualLength - gapAcks*2) / 4
	if gapAcks > int(sc.NumGapACKs) {
		gapAcks = int(sc.NumGapACKs)
	}
	if dupTSNs > int(sc.NumDuplicateTSNs) {
		dupTSNs = int(sc.NumDuplicateTSNs)
	}
	sc.GapACKs = make([]uint16, 0, gapAcks)
	sc.DuplicateTSNs = make([]uint32, 0, dupTSNs)
	bytesRemaining := data[16:]
	for i := 0; i < int(sc.NumGapACKs); i++ {
		sc.GapACKs = append(sc.GapACKs, binary.BigEndian.Uint16(bytesRemaining[:2]))
		bytesRemaining = bytesRemaining[2:]
	}
	for i := 0; i < int(sc.NumDuplicateTSNs); i++ {
		sc.DuplicateTSNs = append(sc.DuplicateTSNs, binary.BigEndian.Uint32(bytesRemaining[:4]))
		bytesRemaining = bytesRemaining[4:]
	}
	p.AddLayer(sc)
	return p.NextDecoder(gopacket.DecodeFunc(decodeWithSCTPChunkTypePrefix))
}

// SerializeTo is for gopacket.SerializableLayer.
func (sc SCTPSack) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	length := 16 + 2*len(sc.GapACKs) + 4*len(sc.DuplicateTSNs)
	bytes, err := b.PrependBytes(roundUpToNearest4(length))
	if err != nil {
		return err
	}
	bytes[0] = uint8(sc.Type)
	bytes[1] = sc.Flags
	binary.BigEndian.PutUint16(bytes[2:4], uint16(length))
	binary.BigEndian.PutUint32(bytes[4:8], sc.CumulativeTSNAck)
	binary.BigEndian.PutUint32(bytes[8:12], sc.AdvertisedReceiverWindowCredit)
	binary.BigEndian.PutUint16(bytes[12:14], uint16(len(sc.GapACKs)))
	binary.BigEndian.PutUint16(bytes[14:16], uint16(len(sc.DuplicateTSNs)))
	for i, v := range sc.GapACKs {
		binary.BigEndian.PutUint16(bytes[16+i*2:], v)
	}
	offset := 16 + 2*len(sc.GapACKs)
	for i, v := range sc.DuplicateTSNs {
		binary.BigEndian.PutUint32(bytes[offset+i*4:], v)
	}
	return nil
}

// SCTPHeartbeatParameter is the parameter type used by SCTP heartbeat and
// heartbeat ack layers.
type SCTPHeartbeatParameter SCTPParameter

// SCTPHeartbeat is the SCTP heartbeat layer, also used for heatbeat ack.
type SCTPHeartbeat struct {
	SCTPChunk
	Parameters []SCTPHeartbeatParameter
}

// LayerType returns gopacket.LayerTypeSCTPHeartbeat.
func (sc *SCTPHeartbeat) LayerType() gopacket.LayerType {
	if sc.Type == SCTPChunkTypeHeartbeatAck {
		return LayerTypeSCTPHeartbeatAck
	}
	// sc.Type == SCTPChunkTypeHeartbeat
	return LayerTypeSCTPHeartbeat
}

func decodeSCTPHeartbeat(data []byte, p gopacket.PacketBuilder) error {
	chunk, err := decodeSCTPChunk(data)
	if err != nil {
		return err
	}
	sc := &SCTPHeartbeat{
		SCTPChunk: chunk,
	}
	paramData := data[4:sc.Length]
	for len(paramData) > 0 {
		p := SCTPHeartbeatParameter(decodeSCTPParameter(paramData))
		paramData = paramData[p.ActualLength:]
		sc.Parameters = append(sc.Parameters, p)
	}
	p.AddLayer(sc)
	return p.NextDecoder(gopacket.DecodeFunc(decodeWithSCTPChunkTypePrefix))
}

// SerializeTo is for gopacket.SerializableLayer.
func (sc SCTPHeartbeat) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	var payload []byte
	for _, param := range sc.Parameters {
		payload = append(payload, SCTPParameter(param).Bytes()...)
	}
	length := 4 + len(payload)

	bytes, err := b.PrependBytes(roundUpToNearest4(length))
	if err != nil {
		return err
	}
	bytes[0] = uint8(sc.Type)
	bytes[1] = sc.Flags
	binary.BigEndian.PutUint16(bytes[2:4], uint16(length))
	copy(bytes[4:], payload)
	return nil
}

// SCTPErrorParameter is the parameter type used by SCTP Abort and Error layers.
type SCTPErrorParameter SCTPParameter

// SCTPError is the SCTP error layer, also used for SCTP aborts.
type SCTPError struct {
	SCTPChunk
	Parameters []SCTPErrorParameter
}

// LayerType returns LayerTypeSCTPAbort or LayerTypeSCTPError.
func (sc *SCTPError) LayerType() gopacket.LayerType {
	if sc.Type == SCTPChunkTypeAbort {
		return LayerTypeSCTPAbort
	}
	// sc.Type == SCTPChunkTypeError
	return LayerTypeSCTPError
}

func decodeSCTPError(data []byte, p gopacket.PacketBuilder) error {
	// remarkably similar to decodeSCTPHeartbeat ;)
	chunk, err := decodeSCTPChunk(data)
	if err != nil {
		return err
	}
	sc := &SCTPError{
		SCTPChunk: chunk,
	}
	paramData := data[4:sc.Length]
	for len(paramData) > 0 {
		p := SCTPErrorParameter(decodeSCTPParameter(paramData))
		paramData = paramData[p.ActualLength:]
		sc.Parameters = append(sc.Parameters, p)
	}
	p.AddLayer(sc)
	return p.NextDecoder(gopacket.DecodeFunc(decodeWithSCTPChunkTypePrefix))
}

// SerializeTo is for gopacket.SerializableLayer.
func (sc SCTPError) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	var payload []byte
	for _, param := range sc.Parameters {
		payload = append(payload, SCTPParameter(param).Bytes()...)
	}
	length := 4 + len(payload)

	bytes, err := b.PrependBytes(roundUpToNearest4(length))
	if err != nil {
		return err
	}
	bytes[0] = uint8(sc.Type)
	bytes[1] = sc.Flags
	binary.BigEndian.PutUint16(bytes[2:4], uint16(length))
	copy(bytes[4:], payload)
	return nil
}

// SCTPShutdown is the SCTP shutdown layer.
type SCTPShutdown struct {
	SCTPChunk
	CumulativeTSNAck uint32
}

// LayerType returns gopacket.LayerTypeSCTPShutdown.
func (sc *SCTPShutdown) LayerType() gopacket.LayerType { return LayerTypeSCTPShutdown }

func decodeSCTPShutdown(data []byte, p gopacket.PacketBuilder) error {
	chunk, err := decodeSCTPChunk(data)
	if err != nil {
		return err
	}
	sc := &SCTPShutdown{
		SCTPChunk:        chunk,
		CumulativeTSNAck: binary.BigEndian.Uint32(data[4:8]),
	}
	p.AddLayer(sc)
	return p.NextDecoder(gopacket.DecodeFunc(decodeWithSCTPChunkTypePrefix))
}

// SerializeTo is for gopacket.SerializableLayer.
func (sc SCTPShutdown) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	bytes, err := b.PrependBytes(8)
	if err != nil {
		return err
	}
	bytes[0] = uint8(sc.Type)
	bytes[1] = sc.Flags
	binary.BigEndian.PutUint16(bytes[2:4], 8)
	binary.BigEndian.PutUint32(bytes[4:8], sc.CumulativeTSNAck)
	return nil
}

// SCTPShutdownAck is the SCTP shutdown layer.
type SCTPShutdownAck struct {
	SCTPChunk
}

// LayerType returns gopacket.LayerTypeSCTPShutdownAck.
func (sc *SCTPShutdownAck) LayerType() gopacket.LayerType { return LayerTypeSCTPShutdownAck }

func decodeSCTPShutdownAck(data []byte, p gopacket.PacketBuilder) error {
	chunk, err := decodeSCTPChunk(data)
	if err != nil {
		return err
	}
	sc := &SCTPShutdownAck{
		SCTPChunk: chunk,
	}
	p.AddLayer(sc)
	return p.NextDecoder(gopacket.DecodeFunc(decodeWithSCTPChunkTypePrefix))
}

// SerializeTo is for gopacket.SerializableLayer.
func (sc SCTPShutdownAck) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	bytes, err := b.PrependBytes(4)
	if err != nil {
		return err
	}
	bytes[0] = uint8(sc.Type)
	bytes[1] = sc.Flags
	binary.BigEndian.PutUint16(bytes[2:4], 4)
	return nil
}

// SCTPCookieEcho is the SCTP Cookie Echo layer.
type SCTPCookieEcho struct {
	SCTPChunk
	Cookie []byte
}

// LayerType returns gopacket.LayerTypeSCTPCookieEcho.
func (sc *SCTPCookieEcho) LayerType() gopacket.LayerType { return LayerTypeSCTPCookieEcho }

func decodeSCTPCookieEcho(data []byte, p gopacket.PacketBuilder) error {
	chunk, err := decodeSCTPChunk(data)
	if err != nil {
		return err
	}
	sc := &SCTPCookieEcho{
		SCTPChunk: chunk,
	}
	sc.Cookie = data[4:sc.Length]
	p.AddLayer(sc)
	return p.NextDecoder(gopacket.DecodeFunc(decodeWithSCTPChunkTypePrefix))
}

// SerializeTo is for gopacket.SerializableLayer.
func (sc SCTPCookieEcho) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	length := 4 + len(sc.Cookie)
	bytes, err := b.PrependBytes(roundUpToNearest4(length))
	if err != nil {
		return err
	}
	bytes[0] = uint8(sc.Type)
	bytes[1] = sc.Flags
	binary.BigEndian.PutUint16(bytes[2:4], uint16(length))
	copy(bytes[4:], sc.Cookie)
	return nil
}

// This struct is used by all empty SCTP chunks (currently CookieAck and
// ShutdownComplete).
type SCTPEmptyLayer struct {
	SCTPChunk
}

// LayerType returns either gopacket.LayerTypeSCTPShutdownComplete or
// LayerTypeSCTPCookieAck.
func (sc *SCTPEmptyLayer) LayerType() gopacket.LayerType {
	if sc.Type == SCTPChunkTypeShutdownComplete {
		return LayerTypeSCTPShutdownComplete
	}
	// sc.Type == SCTPChunkTypeCookieAck
	return LayerTypeSCTPCookieAck
}

func decodeSCTPEmptyLayer(data []byte, p gopacket.PacketBuilder) error {
	chunk, err := decodeSCTPChunk(data)
	if err != nil {
		return err
	}
	sc := &SCTPEmptyLayer{
		SCTPChunk: chunk,
	}
	p.AddLayer(sc)
	return p.NextDecoder(gopacket.DecodeFunc(decodeWithSCTPChunkTypePrefix))
}

// SerializeTo is for gopacket.SerializableLayer.
func (sc SCTPEmptyLayer) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	bytes, err := b.PrependBytes(4)
	if err != nil {
		return err
	}
	bytes[0] = uint8(sc.Type)
	bytes[1] = sc.Flags
	binary.BigEndian.PutUint16(bytes[2:4], 4)
	return nil
}
