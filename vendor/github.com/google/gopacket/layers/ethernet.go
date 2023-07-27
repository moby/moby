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
	"net"
)

// EthernetBroadcast is the broadcast MAC address used by Ethernet.
var EthernetBroadcast = net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

// Ethernet is the layer for Ethernet frame headers.
type Ethernet struct {
	BaseLayer
	SrcMAC, DstMAC net.HardwareAddr
	EthernetType   EthernetType
	// Length is only set if a length field exists within this header.  Ethernet
	// headers follow two different standards, one that uses an EthernetType, the
	// other which defines a length the follows with a LLC header (802.3).  If the
	// former is the case, we set EthernetType and Length stays 0.  In the latter
	// case, we set Length and EthernetType = EthernetTypeLLC.
	Length uint16
}

// LayerType returns LayerTypeEthernet
func (e *Ethernet) LayerType() gopacket.LayerType { return LayerTypeEthernet }

func (e *Ethernet) LinkFlow() gopacket.Flow {
	return gopacket.NewFlow(EndpointMAC, e.SrcMAC, e.DstMAC)
}

func (eth *Ethernet) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 14 {
		return errors.New("Ethernet packet too small")
	}
	eth.DstMAC = net.HardwareAddr(data[0:6])
	eth.SrcMAC = net.HardwareAddr(data[6:12])
	eth.EthernetType = EthernetType(binary.BigEndian.Uint16(data[12:14]))
	eth.BaseLayer = BaseLayer{data[:14], data[14:]}
	eth.Length = 0
	if eth.EthernetType < 0x0600 {
		eth.Length = uint16(eth.EthernetType)
		eth.EthernetType = EthernetTypeLLC
		if cmp := len(eth.Payload) - int(eth.Length); cmp < 0 {
			df.SetTruncated()
		} else if cmp > 0 {
			// Strip off bytes at the end, since we have too many bytes
			eth.Payload = eth.Payload[:len(eth.Payload)-cmp]
		}
		//	fmt.Println(eth)
	}
	return nil
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (eth *Ethernet) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	if len(eth.DstMAC) != 6 {
		return fmt.Errorf("invalid dst MAC: %v", eth.DstMAC)
	}
	if len(eth.SrcMAC) != 6 {
		return fmt.Errorf("invalid src MAC: %v", eth.SrcMAC)
	}
	payload := b.Bytes()
	bytes, err := b.PrependBytes(14)
	if err != nil {
		return err
	}
	copy(bytes, eth.DstMAC)
	copy(bytes[6:], eth.SrcMAC)
	if eth.Length != 0 || eth.EthernetType == EthernetTypeLLC {
		if opts.FixLengths {
			eth.Length = uint16(len(payload))
		}
		if eth.EthernetType != EthernetTypeLLC {
			return fmt.Errorf("ethernet type %v not compatible with length value %v", eth.EthernetType, eth.Length)
		} else if eth.Length > 0x0600 {
			return fmt.Errorf("invalid ethernet length %v", eth.Length)
		}
		binary.BigEndian.PutUint16(bytes[12:], eth.Length)
	} else {
		binary.BigEndian.PutUint16(bytes[12:], uint16(eth.EthernetType))
	}
	length := len(b.Bytes())
	if length < 60 {
		// Pad out to 60 bytes.
		padding, err := b.AppendBytes(60 - length)
		if err != nil {
			return err
		}
		copy(padding, lotsOfZeros[:])
	}
	return nil
}

func (eth *Ethernet) CanDecode() gopacket.LayerClass {
	return LayerTypeEthernet
}

func (eth *Ethernet) NextLayerType() gopacket.LayerType {
	return eth.EthernetType.LayerType()
}

func decodeEthernet(data []byte, p gopacket.PacketBuilder) error {
	eth := &Ethernet{}
	err := eth.DecodeFromBytes(data, p)
	if err != nil {
		return err
	}
	p.AddLayer(eth)
	p.SetLinkLayer(eth)
	return p.NextDecoder(eth.EthernetType)
}
