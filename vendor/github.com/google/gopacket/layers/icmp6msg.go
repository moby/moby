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
	"net"
	"time"

	"github.com/google/gopacket"
)

// Based on RFC 4861

// ICMPv6Opt indicate how to decode the data associated with each ICMPv6Option.
type ICMPv6Opt uint8

const (
	_ ICMPv6Opt = iota

	// ICMPv6OptSourceAddress contains the link-layer address of the sender of
	// the packet. It is used in the Neighbor Solicitation, Router
	// Solicitation, and Router Advertisement packets. Must be ignored for other
	// Neighbor discovery messages.
	ICMPv6OptSourceAddress

	// ICMPv6OptTargetAddress contains the link-layer address of the target. It
	// is used in Neighbor Advertisement and Redirect packets. Must be ignored
	// for other Neighbor discovery messages.
	ICMPv6OptTargetAddress

	// ICMPv6OptPrefixInfo provides hosts with on-link prefixes and prefixes
	// for Address Autoconfiguration. The Prefix Information option appears in
	// Router Advertisement packets and MUST be silently ignored for other
	// messages.
	ICMPv6OptPrefixInfo

	// ICMPv6OptRedirectedHeader is used in Redirect messages and contains all
	// or part of the packet that is being redirected.
	ICMPv6OptRedirectedHeader

	// ICMPv6OptMTU is used in Router Advertisement messages to ensure that all
	// nodes on a link use the same MTU value in those cases where the link MTU
	// is not well known. This option MUST be silently ignored for other
	// Neighbor Discovery messages.
	ICMPv6OptMTU
)

// ICMPv6Echo represents the structure of a ping.
type ICMPv6Echo struct {
	BaseLayer
	Identifier uint16
	SeqNumber  uint16
}

// ICMPv6RouterSolicitation is sent by hosts to find routers.
type ICMPv6RouterSolicitation struct {
	BaseLayer
	Options ICMPv6Options
}

// ICMPv6RouterAdvertisement is sent by routers in response to Solicitation.
type ICMPv6RouterAdvertisement struct {
	BaseLayer
	HopLimit       uint8
	Flags          uint8
	RouterLifetime uint16
	ReachableTime  uint32
	RetransTimer   uint32
	Options        ICMPv6Options
}

// ICMPv6NeighborSolicitation is sent to request the link-layer address of a
// target node.
type ICMPv6NeighborSolicitation struct {
	BaseLayer
	TargetAddress net.IP
	Options       ICMPv6Options
}

// ICMPv6NeighborAdvertisement is sent by nodes in response to Solicitation.
type ICMPv6NeighborAdvertisement struct {
	BaseLayer
	Flags         uint8
	TargetAddress net.IP
	Options       ICMPv6Options
}

// ICMPv6Redirect is sent by routers to inform hosts of a better first-hop node
// on the path to a destination.
type ICMPv6Redirect struct {
	BaseLayer
	TargetAddress      net.IP
	DestinationAddress net.IP
	Options            ICMPv6Options
}

// ICMPv6Option contains the type and data for a single option.
type ICMPv6Option struct {
	Type ICMPv6Opt
	Data []byte
}

// ICMPv6Options is a slice of ICMPv6Option.
type ICMPv6Options []ICMPv6Option

func (i ICMPv6Opt) String() string {
	switch i {
	case ICMPv6OptSourceAddress:
		return "SourceAddress"
	case ICMPv6OptTargetAddress:
		return "TargetAddress"
	case ICMPv6OptPrefixInfo:
		return "PrefixInfo"
	case ICMPv6OptRedirectedHeader:
		return "RedirectedHeader"
	case ICMPv6OptMTU:
		return "MTU"
	default:
		return fmt.Sprintf("Unknown(%d)", i)
	}
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (i *ICMPv6Echo) CanDecode() gopacket.LayerClass {
	return LayerTypeICMPv6Echo
}

// LayerType returns LayerTypeICMPv6Echo.
func (i *ICMPv6Echo) LayerType() gopacket.LayerType {
	return LayerTypeICMPv6Echo
}

// NextLayerType returns the layer type contained by this DecodingLayer.
func (i *ICMPv6Echo) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

// DecodeFromBytes decodes the given bytes into this layer.
func (i *ICMPv6Echo) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 4 {
		df.SetTruncated()
		return errors.New("ICMP layer less then 4 bytes for ICMPv6 Echo")
	}
	i.Identifier = binary.BigEndian.Uint16(data[0:2])
	i.SeqNumber = binary.BigEndian.Uint16(data[2:4])

	return nil
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (i *ICMPv6Echo) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	buf, err := b.PrependBytes(4)
	if err != nil {
		return err
	}

	binary.BigEndian.PutUint16(buf, i.Identifier)
	binary.BigEndian.PutUint16(buf[2:], i.SeqNumber)
	return nil
}

// LayerType returns LayerTypeICMPv6.
func (i *ICMPv6RouterSolicitation) LayerType() gopacket.LayerType {
	return LayerTypeICMPv6RouterSolicitation
}

// NextLayerType returns the layer type contained by this DecodingLayer.
func (i *ICMPv6RouterSolicitation) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

// DecodeFromBytes decodes the given bytes into this layer.
func (i *ICMPv6RouterSolicitation) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	// first 4 bytes are reserved followed by options
	if len(data) < 4 {
		df.SetTruncated()
		return errors.New("ICMP layer less then 4 bytes for ICMPv6 router solicitation")
	}

	// truncate old options
	i.Options = i.Options[:0]

	return i.Options.DecodeFromBytes(data[4:], df)
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (i *ICMPv6RouterSolicitation) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	if err := i.Options.SerializeTo(b, opts); err != nil {
		return err
	}

	buf, err := b.PrependBytes(4)
	if err != nil {
		return err
	}

	copy(buf, lotsOfZeros[:4])
	return nil
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (i *ICMPv6RouterSolicitation) CanDecode() gopacket.LayerClass {
	return LayerTypeICMPv6RouterSolicitation
}

// LayerType returns LayerTypeICMPv6RouterAdvertisement.
func (i *ICMPv6RouterAdvertisement) LayerType() gopacket.LayerType {
	return LayerTypeICMPv6RouterAdvertisement
}

// NextLayerType returns the layer type contained by this DecodingLayer.
func (i *ICMPv6RouterAdvertisement) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

// DecodeFromBytes decodes the given bytes into this layer.
func (i *ICMPv6RouterAdvertisement) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 12 {
		df.SetTruncated()
		return errors.New("ICMP layer less then 12 bytes for ICMPv6 router advertisement")
	}

	i.HopLimit = uint8(data[0])
	// M, O bit followed by 6 reserved bits
	i.Flags = uint8(data[1])
	i.RouterLifetime = binary.BigEndian.Uint16(data[2:4])
	i.ReachableTime = binary.BigEndian.Uint32(data[4:8])
	i.RetransTimer = binary.BigEndian.Uint32(data[8:12])
	i.BaseLayer = BaseLayer{data, nil} // assume no payload

	// truncate old options
	i.Options = i.Options[:0]

	return i.Options.DecodeFromBytes(data[12:], df)
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (i *ICMPv6RouterAdvertisement) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	if err := i.Options.SerializeTo(b, opts); err != nil {
		return err
	}

	buf, err := b.PrependBytes(12)
	if err != nil {
		return err
	}

	buf[0] = byte(i.HopLimit)
	buf[1] = byte(i.Flags)
	binary.BigEndian.PutUint16(buf[2:], i.RouterLifetime)
	binary.BigEndian.PutUint32(buf[4:], i.ReachableTime)
	binary.BigEndian.PutUint32(buf[8:], i.RetransTimer)
	return nil
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (i *ICMPv6RouterAdvertisement) CanDecode() gopacket.LayerClass {
	return LayerTypeICMPv6RouterAdvertisement
}

// ManagedAddressConfig is true when addresses are available via DHCPv6. If
// set, the OtherConfig flag is redundant.
func (i *ICMPv6RouterAdvertisement) ManagedAddressConfig() bool {
	return i.Flags&0x80 != 0
}

// OtherConfig is true when there is other configuration information available
// via DHCPv6. For example, DNS-related information.
func (i *ICMPv6RouterAdvertisement) OtherConfig() bool {
	return i.Flags&0x40 != 0
}

// LayerType returns LayerTypeICMPv6NeighborSolicitation.
func (i *ICMPv6NeighborSolicitation) LayerType() gopacket.LayerType {
	return LayerTypeICMPv6NeighborSolicitation
}

// NextLayerType returns the layer type contained by this DecodingLayer.
func (i *ICMPv6NeighborSolicitation) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

// DecodeFromBytes decodes the given bytes into this layer.
func (i *ICMPv6NeighborSolicitation) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 20 {
		df.SetTruncated()
		return errors.New("ICMP layer less then 20 bytes for ICMPv6 neighbor solicitation")
	}

	i.TargetAddress = net.IP(data[4:20])
	i.BaseLayer = BaseLayer{data, nil} // assume no payload

	// truncate old options
	i.Options = i.Options[:0]

	return i.Options.DecodeFromBytes(data[20:], df)
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (i *ICMPv6NeighborSolicitation) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	if err := i.Options.SerializeTo(b, opts); err != nil {
		return err
	}

	buf, err := b.PrependBytes(20)
	if err != nil {
		return err
	}

	copy(buf, lotsOfZeros[:4])
	copy(buf[4:], i.TargetAddress)
	return nil
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (i *ICMPv6NeighborSolicitation) CanDecode() gopacket.LayerClass {
	return LayerTypeICMPv6NeighborSolicitation
}

// LayerType returns LayerTypeICMPv6NeighborAdvertisement.
func (i *ICMPv6NeighborAdvertisement) LayerType() gopacket.LayerType {
	return LayerTypeICMPv6NeighborAdvertisement
}

// NextLayerType returns the layer type contained by this DecodingLayer.
func (i *ICMPv6NeighborAdvertisement) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

// DecodeFromBytes decodes the given bytes into this layer.
func (i *ICMPv6NeighborAdvertisement) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 20 {
		df.SetTruncated()
		return errors.New("ICMP layer less then 20 bytes for ICMPv6 neighbor advertisement")
	}

	i.Flags = uint8(data[0])
	i.TargetAddress = net.IP(data[4:20])
	i.BaseLayer = BaseLayer{data, nil} // assume no payload

	// truncate old options
	i.Options = i.Options[:0]

	return i.Options.DecodeFromBytes(data[20:], df)
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (i *ICMPv6NeighborAdvertisement) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	if err := i.Options.SerializeTo(b, opts); err != nil {
		return err
	}

	buf, err := b.PrependBytes(20)
	if err != nil {
		return err
	}

	buf[0] = byte(i.Flags)
	copy(buf[1:], lotsOfZeros[:3])
	copy(buf[4:], i.TargetAddress)
	return nil
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (i *ICMPv6NeighborAdvertisement) CanDecode() gopacket.LayerClass {
	return LayerTypeICMPv6NeighborAdvertisement
}

// Router indicates whether the sender is a router or not.
func (i *ICMPv6NeighborAdvertisement) Router() bool {
	return i.Flags&0x80 != 0
}

// Solicited indicates whether the advertisement was solicited or not.
func (i *ICMPv6NeighborAdvertisement) Solicited() bool {
	return i.Flags&0x40 != 0
}

// Override indicates whether the advertisement should Override an existing
// cache entry.
func (i *ICMPv6NeighborAdvertisement) Override() bool {
	return i.Flags&0x20 != 0
}

// LayerType returns LayerTypeICMPv6Redirect.
func (i *ICMPv6Redirect) LayerType() gopacket.LayerType {
	return LayerTypeICMPv6Redirect
}

// NextLayerType returns the layer type contained by this DecodingLayer.
func (i *ICMPv6Redirect) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

// DecodeFromBytes decodes the given bytes into this layer.
func (i *ICMPv6Redirect) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 36 {
		df.SetTruncated()
		return errors.New("ICMP layer less then 36 bytes for ICMPv6 redirect")
	}

	i.TargetAddress = net.IP(data[4:20])
	i.DestinationAddress = net.IP(data[20:36])
	i.BaseLayer = BaseLayer{data, nil} // assume no payload

	// truncate old options
	i.Options = i.Options[:0]

	return i.Options.DecodeFromBytes(data[36:], df)
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (i *ICMPv6Redirect) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	if err := i.Options.SerializeTo(b, opts); err != nil {
		return err
	}

	buf, err := b.PrependBytes(36)
	if err != nil {
		return err
	}

	copy(buf, lotsOfZeros[:4])
	copy(buf[4:], i.TargetAddress)
	copy(buf[20:], i.DestinationAddress)
	return nil
}

// CanDecode returns the set of layer types that this DecodingLayer can decode.
func (i *ICMPv6Redirect) CanDecode() gopacket.LayerClass {
	return LayerTypeICMPv6Redirect
}

func (i ICMPv6Option) String() string {
	hd := hex.EncodeToString(i.Data)
	if len(hd) > 0 {
		hd = " 0x" + hd
	}

	switch i.Type {
	case ICMPv6OptSourceAddress, ICMPv6OptTargetAddress:
		return fmt.Sprintf("ICMPv6Option(%s:%v)",
			i.Type,
			net.HardwareAddr(i.Data))
	case ICMPv6OptPrefixInfo:
		if len(i.Data) == 30 {
			prefixLen := uint8(i.Data[0])
			onLink := (i.Data[1]&0x80 != 0)
			autonomous := (i.Data[1]&0x40 != 0)
			validLifetime := time.Duration(binary.BigEndian.Uint32(i.Data[2:6])) * time.Second
			preferredLifetime := time.Duration(binary.BigEndian.Uint32(i.Data[6:10])) * time.Second

			prefix := net.IP(i.Data[14:])

			return fmt.Sprintf("ICMPv6Option(%s:%v/%v:%t:%t:%v:%v)",
				i.Type,
				prefix, prefixLen,
				onLink, autonomous,
				validLifetime, preferredLifetime)
		}
	case ICMPv6OptRedirectedHeader:
		// could invoke IP decoder on data... probably best not to
		break
	case ICMPv6OptMTU:
		if len(i.Data) == 6 {
			return fmt.Sprintf("ICMPv6Option(%s:%v)",
				i.Type,
				binary.BigEndian.Uint32(i.Data[2:]))
		}

	}
	return fmt.Sprintf("ICMPv6Option(%s:%s)", i.Type, hd)
}

// DecodeFromBytes decodes the given bytes into this layer.
func (i *ICMPv6Options) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	for len(data) > 0 {
		if len(data) < 2 {
			df.SetTruncated()
			return errors.New("ICMP layer less then 2 bytes for ICMPv6 message option")
		}

		// unit is 8 octets, convert to bytes
		length := int(data[1]) * 8

		if length == 0 {
			df.SetTruncated()
			return errors.New("ICMPv6 message option with length 0")
		}

		if len(data) < length {
			df.SetTruncated()
			return fmt.Errorf("ICMP layer only %v bytes for ICMPv6 message option with length %v", len(data), length)
		}

		o := ICMPv6Option{
			Type: ICMPv6Opt(data[0]),
			Data: data[2:length],
		}

		// chop off option we just consumed
		data = data[length:]

		*i = append(*i, o)
	}

	return nil
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
// See the docs for gopacket.SerializableLayer for more info.
func (i *ICMPv6Options) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	for _, opt := range []ICMPv6Option(*i) {
		length := len(opt.Data) + 2
		buf, err := b.PrependBytes(length)
		if err != nil {
			return err
		}

		buf[0] = byte(opt.Type)
		buf[1] = byte(length / 8)
		copy(buf[2:], opt.Data)
	}

	return nil
}

func decodeICMPv6Echo(data []byte, p gopacket.PacketBuilder) error {
	i := &ICMPv6Echo{}
	return decodingLayerDecoder(i, data, p)
}

func decodeICMPv6RouterSolicitation(data []byte, p gopacket.PacketBuilder) error {
	i := &ICMPv6RouterSolicitation{}
	return decodingLayerDecoder(i, data, p)
}

func decodeICMPv6RouterAdvertisement(data []byte, p gopacket.PacketBuilder) error {
	i := &ICMPv6RouterAdvertisement{}
	return decodingLayerDecoder(i, data, p)
}

func decodeICMPv6NeighborSolicitation(data []byte, p gopacket.PacketBuilder) error {
	i := &ICMPv6NeighborSolicitation{}
	return decodingLayerDecoder(i, data, p)
}

func decodeICMPv6NeighborAdvertisement(data []byte, p gopacket.PacketBuilder) error {
	i := &ICMPv6NeighborAdvertisement{}
	return decodingLayerDecoder(i, data, p)
}

func decodeICMPv6Redirect(data []byte, p gopacket.PacketBuilder) error {
	i := &ICMPv6Redirect{}
	return decodingLayerDecoder(i, data, p)
}
