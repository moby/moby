// Copyright 2012 Google, Inc. All rights reserved.
// Copyright 2009-2011 Andreas Krennmair. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/google/gopacket"
)

type IPv4Flag uint8

const (
	IPv4EvilBit       IPv4Flag = 1 << 2 // http://tools.ietf.org/html/rfc3514 ;)
	IPv4DontFragment  IPv4Flag = 1 << 1
	IPv4MoreFragments IPv4Flag = 1 << 0
)

func (f IPv4Flag) String() string {
	var s []string
	if f&IPv4EvilBit != 0 {
		s = append(s, "Evil")
	}
	if f&IPv4DontFragment != 0 {
		s = append(s, "DF")
	}
	if f&IPv4MoreFragments != 0 {
		s = append(s, "MF")
	}
	return strings.Join(s, "|")
}

// IPv4 is the header of an IP packet.
type IPv4 struct {
	BaseLayer
	Version    uint8
	IHL        uint8
	TOS        uint8
	Length     uint16
	Id         uint16
	Flags      IPv4Flag
	FragOffset uint16
	TTL        uint8
	Protocol   IPProtocol
	Checksum   uint16
	SrcIP      net.IP
	DstIP      net.IP
	Options    []IPv4Option
	Padding    []byte
}

// LayerType returns LayerTypeIPv4
func (i *IPv4) LayerType() gopacket.LayerType { return LayerTypeIPv4 }
func (i *IPv4) NetworkFlow() gopacket.Flow {
	return gopacket.NewFlow(EndpointIPv4, i.SrcIP, i.DstIP)
}

type IPv4Option struct {
	OptionType   uint8
	OptionLength uint8
	OptionData   []byte
}

func (i IPv4Option) String() string {
	return fmt.Sprintf("IPv4Option(%v:%v)", i.OptionType, i.OptionData)
}

// for the current ipv4 options, return the number of bytes (including
// padding that the options used)
func (ip *IPv4) getIPv4OptionSize() uint8 {
	optionSize := uint8(0)
	for _, opt := range ip.Options {
		switch opt.OptionType {
		case 0:
			// this is the end of option lists
			optionSize++
		case 1:
			// this is the padding
			optionSize++
		default:
			optionSize += opt.OptionLength

		}
	}
	// make sure the options are aligned to 32 bit boundary
	if (optionSize % 4) != 0 {
		optionSize += 4 - (optionSize % 4)
	}
	return optionSize
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
func (ip *IPv4) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	optionLength := ip.getIPv4OptionSize()
	bytes, err := b.PrependBytes(20 + int(optionLength))
	if err != nil {
		return err
	}
	if opts.FixLengths {
		ip.IHL = 5 + (optionLength / 4)
		ip.Length = uint16(len(b.Bytes()))
	}
	bytes[0] = (ip.Version << 4) | ip.IHL
	bytes[1] = ip.TOS
	binary.BigEndian.PutUint16(bytes[2:], ip.Length)
	binary.BigEndian.PutUint16(bytes[4:], ip.Id)
	binary.BigEndian.PutUint16(bytes[6:], ip.flagsfrags())
	bytes[8] = ip.TTL
	bytes[9] = byte(ip.Protocol)
	if err := ip.AddressTo4(); err != nil {
		return err
	}
	copy(bytes[12:16], ip.SrcIP)
	copy(bytes[16:20], ip.DstIP)

	curLocation := 20
	// Now, we will encode the options
	for _, opt := range ip.Options {
		switch opt.OptionType {
		case 0:
			// this is the end of option lists
			bytes[curLocation] = 0
			curLocation++
		case 1:
			// this is the padding
			bytes[curLocation] = 1
			curLocation++
		default:
			bytes[curLocation] = opt.OptionType
			bytes[curLocation+1] = opt.OptionLength

			// sanity checking to protect us from buffer overrun
			if len(opt.OptionData) > int(opt.OptionLength-2) {
				return errors.New("option length is smaller than length of option data")
			}
			copy(bytes[curLocation+2:curLocation+int(opt.OptionLength)], opt.OptionData)
			curLocation += int(opt.OptionLength)
		}
	}

	if opts.ComputeChecksums {
		ip.Checksum = checksum(bytes)
	}
	binary.BigEndian.PutUint16(bytes[10:], ip.Checksum)
	return nil
}

func checksum(bytes []byte) uint16 {
	// Clear checksum bytes
	bytes[10] = 0
	bytes[11] = 0

	// Compute checksum
	var csum uint32
	for i := 0; i < len(bytes); i += 2 {
		csum += uint32(bytes[i]) << 8
		csum += uint32(bytes[i+1])
	}
	for {
		// Break when sum is less or equals to 0xFFFF
		if csum <= 65535 {
			break
		}
		// Add carry to the sum
		csum = (csum >> 16) + uint32(uint16(csum))
	}
	// Flip all the bits
	return ^uint16(csum)
}

func (ip *IPv4) flagsfrags() (ff uint16) {
	ff |= uint16(ip.Flags) << 13
	ff |= ip.FragOffset
	return
}

// DecodeFromBytes decodes the given bytes into this layer.
func (ip *IPv4) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 20 {
		df.SetTruncated()
		return fmt.Errorf("Invalid ip4 header. Length %d less than 20", len(data))
	}
	flagsfrags := binary.BigEndian.Uint16(data[6:8])

	ip.Version = uint8(data[0]) >> 4
	ip.IHL = uint8(data[0]) & 0x0F
	ip.TOS = data[1]
	ip.Length = binary.BigEndian.Uint16(data[2:4])
	ip.Id = binary.BigEndian.Uint16(data[4:6])
	ip.Flags = IPv4Flag(flagsfrags >> 13)
	ip.FragOffset = flagsfrags & 0x1FFF
	ip.TTL = data[8]
	ip.Protocol = IPProtocol(data[9])
	ip.Checksum = binary.BigEndian.Uint16(data[10:12])
	ip.SrcIP = data[12:16]
	ip.DstIP = data[16:20]
	ip.Options = ip.Options[:0]
	ip.Padding = nil
	// Set up an initial guess for contents/payload... we'll reset these soon.
	ip.BaseLayer = BaseLayer{Contents: data}

	// This code is added for the following enviroment:
	// * Windows 10 with TSO option activated. ( tested on Hyper-V, RealTek ethernet driver )
	if ip.Length == 0 {
		// If using TSO(TCP Segmentation Offload), length is zero.
		// The actual packet length is the length of data.
		ip.Length = uint16(len(data))
	}

	if ip.Length < 20 {
		return fmt.Errorf("Invalid (too small) IP length (%d < 20)", ip.Length)
	} else if ip.IHL < 5 {
		return fmt.Errorf("Invalid (too small) IP header length (%d < 5)", ip.IHL)
	} else if int(ip.IHL*4) > int(ip.Length) {
		return fmt.Errorf("Invalid IP header length > IP length (%d > %d)", ip.IHL, ip.Length)
	}
	if cmp := len(data) - int(ip.Length); cmp > 0 {
		data = data[:ip.Length]
	} else if cmp < 0 {
		df.SetTruncated()
		if int(ip.IHL)*4 > len(data) {
			return errors.New("Not all IP header bytes available")
		}
	}
	ip.Contents = data[:ip.IHL*4]
	ip.Payload = data[ip.IHL*4:]
	// From here on, data contains the header options.
	data = data[20 : ip.IHL*4]
	// Pull out IP options
	for len(data) > 0 {
		if ip.Options == nil {
			// Pre-allocate to avoid growing the slice too much.
			ip.Options = make([]IPv4Option, 0, 4)
		}
		opt := IPv4Option{OptionType: data[0]}
		switch opt.OptionType {
		case 0: // End of options
			opt.OptionLength = 1
			ip.Options = append(ip.Options, opt)
			ip.Padding = data[1:]
			return nil
		case 1: // 1 byte padding
			opt.OptionLength = 1
			data = data[1:]
			ip.Options = append(ip.Options, opt)
		default:
			if len(data) < 2 {
				df.SetTruncated()
				return fmt.Errorf("Invalid ip4 option length. Length %d less than 2", len(data))
			}
			opt.OptionLength = data[1]
			if len(data) < int(opt.OptionLength) {
				df.SetTruncated()
				return fmt.Errorf("IP option length exceeds remaining IP header size, option type %v length %v", opt.OptionType, opt.OptionLength)
			}
			if opt.OptionLength <= 2 {
				return fmt.Errorf("Invalid IP option type %v length %d. Must be greater than 2", opt.OptionType, opt.OptionLength)
			}
			opt.OptionData = data[2:opt.OptionLength]
			data = data[opt.OptionLength:]
			ip.Options = append(ip.Options, opt)
		}
	}
	return nil
}

func (i *IPv4) CanDecode() gopacket.LayerClass {
	return LayerTypeIPv4
}

func (i *IPv4) NextLayerType() gopacket.LayerType {
	if i.Flags&IPv4MoreFragments != 0 || i.FragOffset != 0 {
		return gopacket.LayerTypeFragment
	}
	return i.Protocol.LayerType()
}

func decodeIPv4(data []byte, p gopacket.PacketBuilder) error {
	ip := &IPv4{}
	err := ip.DecodeFromBytes(data, p)
	p.AddLayer(ip)
	p.SetNetworkLayer(ip)
	if err != nil {
		return err
	}
	return p.NextDecoder(ip.NextLayerType())
}

func checkIPv4Address(addr net.IP) (net.IP, error) {
	if c := addr.To4(); c != nil {
		return c, nil
	}
	if len(addr) == net.IPv6len {
		return nil, errors.New("address is IPv6")
	}
	return nil, fmt.Errorf("wrong length of %d bytes instead of %d", len(addr), net.IPv4len)
}

func (ip *IPv4) AddressTo4() error {
	var src, dst net.IP

	if addr, err := checkIPv4Address(ip.SrcIP); err != nil {
		return fmt.Errorf("Invalid source IPv4 address (%s)", err)
	} else {
		src = addr
	}
	if addr, err := checkIPv4Address(ip.DstIP); err != nil {
		return fmt.Errorf("Invalid destination IPv4 address (%s)", err)
	} else {
		dst = addr
	}
	ip.SrcIP = src
	ip.DstIP = dst
	return nil
}
