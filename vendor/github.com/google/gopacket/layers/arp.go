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

	"github.com/google/gopacket"
)

// Potential values for ARP.Operation.
const (
	ARPRequest = 1
	ARPReply   = 2
)

// ARP is a ARP packet header.
type ARP struct {
	BaseLayer
	AddrType          LinkType
	Protocol          EthernetType
	HwAddressSize     uint8
	ProtAddressSize   uint8
	Operation         uint16
	SourceHwAddress   []byte
	SourceProtAddress []byte
	DstHwAddress      []byte
	DstProtAddress    []byte
}

// LayerType returns LayerTypeARP
func (arp *ARP) LayerType() gopacket.LayerType { return LayerTypeARP }

// DecodeFromBytes decodes the given bytes into this layer.
func (arp *ARP) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 8 {
		df.SetTruncated()
		return fmt.Errorf("ARP length %d too short", len(data))
	}
	arp.AddrType = LinkType(binary.BigEndian.Uint16(data[0:2]))
	arp.Protocol = EthernetType(binary.BigEndian.Uint16(data[2:4]))
	arp.HwAddressSize = data[4]
	arp.ProtAddressSize = data[5]
	arp.Operation = binary.BigEndian.Uint16(data[6:8])
	arpLength := 8 + 2*arp.HwAddressSize + 2*arp.ProtAddressSize
	if len(data) < int(arpLength) {
		df.SetTruncated()
		return fmt.Errorf("ARP length %d too short, %d expected", len(data), arpLength)
	}
	arp.SourceHwAddress = data[8 : 8+arp.HwAddressSize]
	arp.SourceProtAddress = data[8+arp.HwAddressSize : 8+arp.HwAddressSize+arp.ProtAddressSize]
	arp.DstHwAddress = data[8+arp.HwAddressSize+arp.ProtAddressSize : 8+2*arp.HwAddressSize+arp.ProtAddressSize]
	arp.DstProtAddress = data[8+2*arp.HwAddressSize+arp.ProtAddressSize : 8+2*arp.HwAddressSize+2*arp.ProtAddressSize]

	arp.Contents = data[:arpLength]
	arp.Payload = data[arpLength:]
	return nil
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (arp *ARP) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	size := 8 + len(arp.SourceHwAddress) + len(arp.SourceProtAddress) + len(arp.DstHwAddress) + len(arp.DstProtAddress)
	bytes, err := b.PrependBytes(size)
	if err != nil {
		return err
	}
	if opts.FixLengths {
		if len(arp.SourceHwAddress) != len(arp.DstHwAddress) {
			return errors.New("mismatched hardware address sizes")
		}
		arp.HwAddressSize = uint8(len(arp.SourceHwAddress))
		if len(arp.SourceProtAddress) != len(arp.DstProtAddress) {
			return errors.New("mismatched prot address sizes")
		}
		arp.ProtAddressSize = uint8(len(arp.SourceProtAddress))
	}
	binary.BigEndian.PutUint16(bytes, uint16(arp.AddrType))
	binary.BigEndian.PutUint16(bytes[2:], uint16(arp.Protocol))
	bytes[4] = arp.HwAddressSize
	bytes[5] = arp.ProtAddressSize
	binary.BigEndian.PutUint16(bytes[6:], arp.Operation)
	start := 8
	for _, addr := range [][]byte{
		arp.SourceHwAddress,
		arp.SourceProtAddress,
		arp.DstHwAddress,
		arp.DstProtAddress,
	} {
		copy(bytes[start:], addr)
		start += len(addr)
	}
	return nil
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (arp *ARP) CanDecode() gopacket.LayerClass {
	return LayerTypeARP
}

// NextLayerType returns the layer type contained by this DecodingLayer.
func (arp *ARP) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

func decodeARP(data []byte, p gopacket.PacketBuilder) error {

	arp := &ARP{}
	return decodingLayerDecoder(arp, data, p)
}
