// Copyright 2012 Google, Inc. All rights reserved.
// Copyright 2009-2011 Andreas Krennmair. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/gopacket"
)

// TCP is the layer for TCP headers.
type TCP struct {
	BaseLayer
	SrcPort, DstPort                           TCPPort
	Seq                                        uint32
	Ack                                        uint32
	DataOffset                                 uint8
	FIN, SYN, RST, PSH, ACK, URG, ECE, CWR, NS bool
	Window                                     uint16
	Checksum                                   uint16
	Urgent                                     uint16
	sPort, dPort                               []byte
	Options                                    []TCPOption
	Padding                                    []byte
	opts                                       [4]TCPOption
	tcpipchecksum
}

// TCPOptionKind represents a TCP option code.
type TCPOptionKind uint8

const (
	TCPOptionKindEndList                         = 0
	TCPOptionKindNop                             = 1
	TCPOptionKindMSS                             = 2  // len = 4
	TCPOptionKindWindowScale                     = 3  // len = 3
	TCPOptionKindSACKPermitted                   = 4  // len = 2
	TCPOptionKindSACK                            = 5  // len = n
	TCPOptionKindEcho                            = 6  // len = 6, obsolete
	TCPOptionKindEchoReply                       = 7  // len = 6, obsolete
	TCPOptionKindTimestamps                      = 8  // len = 10
	TCPOptionKindPartialOrderConnectionPermitted = 9  // len = 2, obsolete
	TCPOptionKindPartialOrderServiceProfile      = 10 // len = 3, obsolete
	TCPOptionKindCC                              = 11 // obsolete
	TCPOptionKindCCNew                           = 12 // obsolete
	TCPOptionKindCCEcho                          = 13 // obsolete
	TCPOptionKindAltChecksum                     = 14 // len = 3, obsolete
	TCPOptionKindAltChecksumData                 = 15 // len = n, obsolete
)

func (k TCPOptionKind) String() string {
	switch k {
	case TCPOptionKindEndList:
		return "EndList"
	case TCPOptionKindNop:
		return "NOP"
	case TCPOptionKindMSS:
		return "MSS"
	case TCPOptionKindWindowScale:
		return "WindowScale"
	case TCPOptionKindSACKPermitted:
		return "SACKPermitted"
	case TCPOptionKindSACK:
		return "SACK"
	case TCPOptionKindEcho:
		return "Echo"
	case TCPOptionKindEchoReply:
		return "EchoReply"
	case TCPOptionKindTimestamps:
		return "Timestamps"
	case TCPOptionKindPartialOrderConnectionPermitted:
		return "PartialOrderConnectionPermitted"
	case TCPOptionKindPartialOrderServiceProfile:
		return "PartialOrderServiceProfile"
	case TCPOptionKindCC:
		return "CC"
	case TCPOptionKindCCNew:
		return "CCNew"
	case TCPOptionKindCCEcho:
		return "CCEcho"
	case TCPOptionKindAltChecksum:
		return "AltChecksum"
	case TCPOptionKindAltChecksumData:
		return "AltChecksumData"
	default:
		return fmt.Sprintf("Unknown(%d)", k)
	}
}

type TCPOption struct {
	OptionType   TCPOptionKind
	OptionLength uint8
	OptionData   []byte
}

func (t TCPOption) String() string {
	hd := hex.EncodeToString(t.OptionData)
	if len(hd) > 0 {
		hd = " 0x" + hd
	}
	switch t.OptionType {
	case TCPOptionKindMSS:
		if len(t.OptionData) >= 2 {
			return fmt.Sprintf("TCPOption(%s:%v%s)",
				t.OptionType,
				binary.BigEndian.Uint16(t.OptionData),
				hd)
		}

	case TCPOptionKindTimestamps:
		if len(t.OptionData) == 8 {
			return fmt.Sprintf("TCPOption(%s:%v/%v%s)",
				t.OptionType,
				binary.BigEndian.Uint32(t.OptionData[:4]),
				binary.BigEndian.Uint32(t.OptionData[4:8]),
				hd)
		}
	}
	return fmt.Sprintf("TCPOption(%s:%s)", t.OptionType, hd)
}

// LayerType returns gopacket.LayerTypeTCP
func (t *TCP) LayerType() gopacket.LayerType { return LayerTypeTCP }

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (t *TCP) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	var optionLength int
	for _, o := range t.Options {
		switch o.OptionType {
		case 0, 1:
			optionLength += 1
		default:
			optionLength += 2 + len(o.OptionData)
		}
	}
	if opts.FixLengths {
		if rem := optionLength % 4; rem != 0 {
			t.Padding = lotsOfZeros[:4-rem]
		}
		t.DataOffset = uint8((len(t.Padding) + optionLength + 20) / 4)
	}
	bytes, err := b.PrependBytes(20 + optionLength + len(t.Padding))
	if err != nil {
		return err
	}
	binary.BigEndian.PutUint16(bytes, uint16(t.SrcPort))
	binary.BigEndian.PutUint16(bytes[2:], uint16(t.DstPort))
	binary.BigEndian.PutUint32(bytes[4:], t.Seq)
	binary.BigEndian.PutUint32(bytes[8:], t.Ack)
	binary.BigEndian.PutUint16(bytes[12:], t.flagsAndOffset())
	binary.BigEndian.PutUint16(bytes[14:], t.Window)
	binary.BigEndian.PutUint16(bytes[18:], t.Urgent)
	start := 20
	for _, o := range t.Options {
		bytes[start] = byte(o.OptionType)
		switch o.OptionType {
		case 0, 1:
			start++
		default:
			if opts.FixLengths {
				o.OptionLength = uint8(len(o.OptionData) + 2)
			}
			bytes[start+1] = o.OptionLength
			copy(bytes[start+2:start+len(o.OptionData)+2], o.OptionData)
			start += len(o.OptionData) + 2
		}
	}
	copy(bytes[start:], t.Padding)
	if opts.ComputeChecksums {
		// zero out checksum bytes in current serialization.
		bytes[16] = 0
		bytes[17] = 0
		csum, err := t.computeChecksum(b.Bytes(), IPProtocolTCP)
		if err != nil {
			return err
		}
		t.Checksum = csum
	}
	binary.BigEndian.PutUint16(bytes[16:], t.Checksum)
	return nil
}

func (t *TCP) ComputeChecksum() (uint16, error) {
	return t.computeChecksum(append(t.Contents, t.Payload...), IPProtocolTCP)
}

func (t *TCP) flagsAndOffset() uint16 {
	f := uint16(t.DataOffset) << 12
	if t.FIN {
		f |= 0x0001
	}
	if t.SYN {
		f |= 0x0002
	}
	if t.RST {
		f |= 0x0004
	}
	if t.PSH {
		f |= 0x0008
	}
	if t.ACK {
		f |= 0x0010
	}
	if t.URG {
		f |= 0x0020
	}
	if t.ECE {
		f |= 0x0040
	}
	if t.CWR {
		f |= 0x0080
	}
	if t.NS {
		f |= 0x0100
	}
	return f
}

func (tcp *TCP) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 20 {
		df.SetTruncated()
		return fmt.Errorf("Invalid TCP header. Length %d less than 20", len(data))
	}
	tcp.SrcPort = TCPPort(binary.BigEndian.Uint16(data[0:2]))
	tcp.sPort = data[0:2]
	tcp.DstPort = TCPPort(binary.BigEndian.Uint16(data[2:4]))
	tcp.dPort = data[2:4]
	tcp.Seq = binary.BigEndian.Uint32(data[4:8])
	tcp.Ack = binary.BigEndian.Uint32(data[8:12])
	tcp.DataOffset = data[12] >> 4
	tcp.FIN = data[13]&0x01 != 0
	tcp.SYN = data[13]&0x02 != 0
	tcp.RST = data[13]&0x04 != 0
	tcp.PSH = data[13]&0x08 != 0
	tcp.ACK = data[13]&0x10 != 0
	tcp.URG = data[13]&0x20 != 0
	tcp.ECE = data[13]&0x40 != 0
	tcp.CWR = data[13]&0x80 != 0
	tcp.NS = data[12]&0x01 != 0
	tcp.Window = binary.BigEndian.Uint16(data[14:16])
	tcp.Checksum = binary.BigEndian.Uint16(data[16:18])
	tcp.Urgent = binary.BigEndian.Uint16(data[18:20])
	if tcp.Options == nil {
		// Pre-allocate to avoid allocating a slice.
		tcp.Options = tcp.opts[:0]
	} else {
		tcp.Options = tcp.Options[:0]
	}
	tcp.Padding = tcp.Padding[:0]
	if tcp.DataOffset < 5 {
		return fmt.Errorf("Invalid TCP data offset %d < 5", tcp.DataOffset)
	}
	dataStart := int(tcp.DataOffset) * 4
	if dataStart > len(data) {
		df.SetTruncated()
		tcp.Payload = nil
		tcp.Contents = data
		return errors.New("TCP data offset greater than packet length")
	}
	tcp.Contents = data[:dataStart]
	tcp.Payload = data[dataStart:]
	// From here on, data points just to the header options.
	data = data[20:dataStart]
OPTIONS:
	for len(data) > 0 {
		tcp.Options = append(tcp.Options, TCPOption{OptionType: TCPOptionKind(data[0])})
		opt := &tcp.Options[len(tcp.Options)-1]
		switch opt.OptionType {
		case TCPOptionKindEndList: // End of options
			opt.OptionLength = 1
			tcp.Padding = data[1:]
			break OPTIONS
		case TCPOptionKindNop: // 1 byte padding
			opt.OptionLength = 1
		default:
			if len(data) < 2 {
				df.SetTruncated()
				return fmt.Errorf("Invalid TCP option length. Length %d less than 2", len(data))
			}
			opt.OptionLength = data[1]
			if opt.OptionLength < 2 {
				return fmt.Errorf("Invalid TCP option length %d < 2", opt.OptionLength)
			} else if int(opt.OptionLength) > len(data) {
				df.SetTruncated()
				return fmt.Errorf("Invalid TCP option length %d exceeds remaining %d bytes", opt.OptionLength, len(data))
			}
			opt.OptionData = data[2:opt.OptionLength]
		}
		data = data[opt.OptionLength:]
	}
	return nil
}

func (t *TCP) CanDecode() gopacket.LayerClass {
	return LayerTypeTCP
}

func (t *TCP) NextLayerType() gopacket.LayerType {
	lt := t.DstPort.LayerType()
	if lt == gopacket.LayerTypePayload {
		lt = t.SrcPort.LayerType()
	}
	return lt
}

func decodeTCP(data []byte, p gopacket.PacketBuilder) error {
	tcp := &TCP{}
	err := tcp.DecodeFromBytes(data, p)
	p.AddLayer(tcp)
	p.SetTransportLayer(tcp)
	if err != nil {
		return err
	}
	if p.DecodeOptions().DecodeStreamsAsDatagrams {
		return p.NextDecoder(tcp.NextLayerType())
	} else {
		return p.NextDecoder(gopacket.LayerTypePayload)
	}
}

func (t *TCP) TransportFlow() gopacket.Flow {
	return gopacket.NewFlow(EndpointTCPPort, t.sPort, t.dPort)
}

// For testing only
func (t *TCP) SetInternalPortsForTesting() {
	t.sPort = make([]byte, 2)
	t.dPort = make([]byte, 2)
	binary.BigEndian.PutUint16(t.sPort, uint16(t.SrcPort))
	binary.BigEndian.PutUint16(t.dPort, uint16(t.DstPort))
}
