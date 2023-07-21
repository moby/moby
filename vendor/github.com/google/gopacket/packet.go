// Copyright 2012 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package gopacket

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"runtime/debug"
	"strings"
	"syscall"
	"time"
)

// CaptureInfo provides standardized information about a packet captured off
// the wire or read from a file.
type CaptureInfo struct {
	// Timestamp is the time the packet was captured, if that is known.
	Timestamp time.Time
	// CaptureLength is the total number of bytes read off of the wire.
	CaptureLength int
	// Length is the size of the original packet.  Should always be >=
	// CaptureLength.
	Length int
	// InterfaceIndex
	InterfaceIndex int
	// The packet source can place ancillary data of various types here.
	// For example, the afpacket source can report the VLAN of captured
	// packets this way.
	AncillaryData []interface{}
}

// PacketMetadata contains metadata for a packet.
type PacketMetadata struct {
	CaptureInfo
	// Truncated is true if packet decoding logic detects that there are fewer
	// bytes in the packet than are detailed in various headers (for example, if
	// the number of bytes in the IPv4 contents/payload is less than IPv4.Length).
	// This is also set automatically for packets captured off the wire if
	// CaptureInfo.CaptureLength < CaptureInfo.Length.
	Truncated bool
}

// Packet is the primary object used by gopacket.  Packets are created by a
// Decoder's Decode call.  A packet is made up of a set of Data, which
// is broken into a number of Layers as it is decoded.
type Packet interface {
	//// Functions for outputting the packet as a human-readable string:
	//// ------------------------------------------------------------------
	// String returns a human-readable string representation of the packet.
	// It uses LayerString on each layer to output the layer.
	String() string
	// Dump returns a verbose human-readable string representation of the packet,
	// including a hex dump of all layers.  It uses LayerDump on each layer to
	// output the layer.
	Dump() string

	//// Functions for accessing arbitrary packet layers:
	//// ------------------------------------------------------------------
	// Layers returns all layers in this packet, computing them as necessary
	Layers() []Layer
	// Layer returns the first layer in this packet of the given type, or nil
	Layer(LayerType) Layer
	// LayerClass returns the first layer in this packet of the given class,
	// or nil.
	LayerClass(LayerClass) Layer

	//// Functions for accessing specific types of packet layers.  These functions
	//// return the first layer of each type found within the packet.
	//// ------------------------------------------------------------------
	// LinkLayer returns the first link layer in the packet
	LinkLayer() LinkLayer
	// NetworkLayer returns the first network layer in the packet
	NetworkLayer() NetworkLayer
	// TransportLayer returns the first transport layer in the packet
	TransportLayer() TransportLayer
	// ApplicationLayer returns the first application layer in the packet
	ApplicationLayer() ApplicationLayer
	// ErrorLayer is particularly useful, since it returns nil if the packet
	// was fully decoded successfully, and non-nil if an error was encountered
	// in decoding and the packet was only partially decoded.  Thus, its output
	// can be used to determine if the entire packet was able to be decoded.
	ErrorLayer() ErrorLayer

	//// Functions for accessing data specific to the packet:
	//// ------------------------------------------------------------------
	// Data returns the set of bytes that make up this entire packet.
	Data() []byte
	// Metadata returns packet metadata associated with this packet.
	Metadata() *PacketMetadata
}

// packet contains all the information we need to fulfill the Packet interface,
// and its two "subclasses" (yes, no such thing in Go, bear with me),
// eagerPacket and lazyPacket, provide eager and lazy decoding logic around the
// various functions needed to access this information.
type packet struct {
	// data contains the entire packet data for a packet
	data []byte
	// initialLayers is space for an initial set of layers already created inside
	// the packet.
	initialLayers [6]Layer
	// layers contains each layer we've already decoded
	layers []Layer
	// last is the last layer added to the packet
	last Layer
	// metadata is the PacketMetadata for this packet
	metadata PacketMetadata

	decodeOptions DecodeOptions

	// Pointers to the various important layers
	link        LinkLayer
	network     NetworkLayer
	transport   TransportLayer
	application ApplicationLayer
	failure     ErrorLayer
}

func (p *packet) SetTruncated() {
	p.metadata.Truncated = true
}

func (p *packet) SetLinkLayer(l LinkLayer) {
	if p.link == nil {
		p.link = l
	}
}

func (p *packet) SetNetworkLayer(l NetworkLayer) {
	if p.network == nil {
		p.network = l
	}
}

func (p *packet) SetTransportLayer(l TransportLayer) {
	if p.transport == nil {
		p.transport = l
	}
}

func (p *packet) SetApplicationLayer(l ApplicationLayer) {
	if p.application == nil {
		p.application = l
	}
}

func (p *packet) SetErrorLayer(l ErrorLayer) {
	if p.failure == nil {
		p.failure = l
	}
}

func (p *packet) AddLayer(l Layer) {
	p.layers = append(p.layers, l)
	p.last = l
}

func (p *packet) DumpPacketData() {
	fmt.Fprint(os.Stderr, p.packetDump())
	os.Stderr.Sync()
}

func (p *packet) Metadata() *PacketMetadata {
	return &p.metadata
}

func (p *packet) Data() []byte {
	return p.data
}

func (p *packet) DecodeOptions() *DecodeOptions {
	return &p.decodeOptions
}

func (p *packet) addFinalDecodeError(err error, stack []byte) {
	fail := &DecodeFailure{err: err, stack: stack}
	if p.last == nil {
		fail.data = p.data
	} else {
		fail.data = p.last.LayerPayload()
	}
	p.AddLayer(fail)
	p.SetErrorLayer(fail)
}

func (p *packet) recoverDecodeError() {
	if !p.decodeOptions.SkipDecodeRecovery {
		if r := recover(); r != nil {
			p.addFinalDecodeError(fmt.Errorf("%v", r), debug.Stack())
		}
	}
}

// LayerString outputs an individual layer as a string.  The layer is output
// in a single line, with no trailing newline.  This function is specifically
// designed to do the right thing for most layers... it follows the following
// rules:
//  * If the Layer has a String function, just output that.
//  * Otherwise, output all exported fields in the layer, recursing into
//    exported slices and structs.
// NOTE:  This is NOT THE SAME AS fmt's "%#v".  %#v will output both exported
// and unexported fields... many times packet layers contain unexported stuff
// that would just mess up the output of the layer, see for example the
// Payload layer and it's internal 'data' field, which contains a large byte
// array that would really mess up formatting.
func LayerString(l Layer) string {
	return fmt.Sprintf("%v\t%s", l.LayerType(), layerString(reflect.ValueOf(l), false, false))
}

// Dumper dumps verbose information on a value.  If a layer type implements
// Dumper, then its LayerDump() string will include the results in its output.
type Dumper interface {
	Dump() string
}

// LayerDump outputs a very verbose string representation of a layer.  Its
// output is a concatenation of LayerString(l) and hex.Dump(l.LayerContents()).
// It contains newlines and ends with a newline.
func LayerDump(l Layer) string {
	var b bytes.Buffer
	b.WriteString(LayerString(l))
	b.WriteByte('\n')
	if d, ok := l.(Dumper); ok {
		dump := d.Dump()
		if dump != "" {
			b.WriteString(dump)
			if dump[len(dump)-1] != '\n' {
				b.WriteByte('\n')
			}
		}
	}
	b.WriteString(hex.Dump(l.LayerContents()))
	return b.String()
}

// layerString outputs, recursively, a layer in a "smart" way.  See docs for
// LayerString for more details.
//
// Params:
//   i - value to write out
//   anonymous:  if we're currently recursing an anonymous member of a struct
//   writeSpace:  if we've already written a value in a struct, and need to
//     write a space before writing more.  This happens when we write various
//     anonymous values, and need to keep writing more.
func layerString(v reflect.Value, anonymous bool, writeSpace bool) string {
	// Let String() functions take precedence.
	if v.CanInterface() {
		if s, ok := v.Interface().(fmt.Stringer); ok {
			return s.String()
		}
	}
	// Reflect, and spit out all the exported fields as key=value.
	switch v.Type().Kind() {
	case reflect.Interface, reflect.Ptr:
		if v.IsNil() {
			return "nil"
		}
		r := v.Elem()
		return layerString(r, anonymous, writeSpace)
	case reflect.Struct:
		var b bytes.Buffer
		typ := v.Type()
		if !anonymous {
			b.WriteByte('{')
		}
		for i := 0; i < v.NumField(); i++ {
			// Check if this is upper-case.
			ftype := typ.Field(i)
			f := v.Field(i)
			if ftype.Anonymous {
				anonStr := layerString(f, true, writeSpace)
				writeSpace = writeSpace || anonStr != ""
				b.WriteString(anonStr)
			} else if ftype.PkgPath == "" { // exported
				if writeSpace {
					b.WriteByte(' ')
				}
				writeSpace = true
				fmt.Fprintf(&b, "%s=%s", typ.Field(i).Name, layerString(f, false, writeSpace))
			}
		}
		if !anonymous {
			b.WriteByte('}')
		}
		return b.String()
	case reflect.Slice:
		var b bytes.Buffer
		b.WriteByte('[')
		if v.Len() > 4 {
			fmt.Fprintf(&b, "..%d..", v.Len())
		} else {
			for j := 0; j < v.Len(); j++ {
				if j != 0 {
					b.WriteString(", ")
				}
				b.WriteString(layerString(v.Index(j), false, false))
			}
		}
		b.WriteByte(']')
		return b.String()
	}
	return fmt.Sprintf("%v", v.Interface())
}

const (
	longBytesLength = 128
)

// LongBytesGoString returns a string representation of the byte slice shortened
// using the format '<type>{<truncated slice> ... (<n> bytes)}' if it
// exceeds a predetermined length. Can be used to avoid filling the display with
// very long byte strings.
func LongBytesGoString(buf []byte) string {
	if len(buf) < longBytesLength {
		return fmt.Sprintf("%#v", buf)
	}
	s := fmt.Sprintf("%#v", buf[:longBytesLength-1])
	s = strings.TrimSuffix(s, "}")
	return fmt.Sprintf("%s ... (%d bytes)}", s, len(buf))
}

func baseLayerString(value reflect.Value) string {
	t := value.Type()
	content := value.Field(0)
	c := make([]byte, content.Len())
	for i := range c {
		c[i] = byte(content.Index(i).Uint())
	}
	payload := value.Field(1)
	p := make([]byte, payload.Len())
	for i := range p {
		p[i] = byte(payload.Index(i).Uint())
	}
	return fmt.Sprintf("%s{Contents:%s, Payload:%s}", t.String(),
		LongBytesGoString(c),
		LongBytesGoString(p))
}

func layerGoString(i interface{}, b *bytes.Buffer) {
	if s, ok := i.(fmt.GoStringer); ok {
		b.WriteString(s.GoString())
		return
	}

	var v reflect.Value
	var ok bool
	if v, ok = i.(reflect.Value); !ok {
		v = reflect.ValueOf(i)
	}
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		if v.Kind() == reflect.Ptr {
			b.WriteByte('&')
		}
		layerGoString(v.Elem().Interface(), b)
	case reflect.Struct:
		t := v.Type()
		b.WriteString(t.String())
		b.WriteByte('{')
		for i := 0; i < v.NumField(); i++ {
			if i > 0 {
				b.WriteString(", ")
			}
			if t.Field(i).Name == "BaseLayer" {
				fmt.Fprintf(b, "BaseLayer:%s", baseLayerString(v.Field(i)))
			} else if v.Field(i).Kind() == reflect.Struct {
				fmt.Fprintf(b, "%s:", t.Field(i).Name)
				layerGoString(v.Field(i), b)
			} else if v.Field(i).Kind() == reflect.Ptr {
				b.WriteByte('&')
				layerGoString(v.Field(i), b)
			} else {
				fmt.Fprintf(b, "%s:%#v", t.Field(i).Name, v.Field(i))
			}
		}
		b.WriteByte('}')
	default:
		fmt.Fprintf(b, "%#v", i)
	}
}

// LayerGoString returns a representation of the layer in Go syntax,
// taking care to shorten "very long" BaseLayer byte slices
func LayerGoString(l Layer) string {
	b := new(bytes.Buffer)
	layerGoString(l, b)
	return b.String()
}

func (p *packet) packetString() string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "PACKET: %d bytes", len(p.Data()))
	if p.metadata.Truncated {
		b.WriteString(", truncated")
	}
	if p.metadata.Length > 0 {
		fmt.Fprintf(&b, ", wire length %d cap length %d", p.metadata.Length, p.metadata.CaptureLength)
	}
	if !p.metadata.Timestamp.IsZero() {
		fmt.Fprintf(&b, " @ %v", p.metadata.Timestamp)
	}
	b.WriteByte('\n')
	for i, l := range p.layers {
		fmt.Fprintf(&b, "- Layer %d (%02d bytes) = %s\n", i+1, len(l.LayerContents()), LayerString(l))
	}
	return b.String()
}

func (p *packet) packetDump() string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "-- FULL PACKET DATA (%d bytes) ------------------------------------\n%s", len(p.data), hex.Dump(p.data))
	for i, l := range p.layers {
		fmt.Fprintf(&b, "--- Layer %d ---\n%s", i+1, LayerDump(l))
	}
	return b.String()
}

// eagerPacket is a packet implementation that does eager decoding.  Upon
// initial construction, it decodes all the layers it can from packet data.
// eagerPacket implements Packet and PacketBuilder.
type eagerPacket struct {
	packet
}

var errNilDecoder = errors.New("NextDecoder passed nil decoder, probably an unsupported decode type")

func (p *eagerPacket) NextDecoder(next Decoder) error {
	if next == nil {
		return errNilDecoder
	}
	if p.last == nil {
		return errors.New("NextDecoder called, but no layers added yet")
	}
	d := p.last.LayerPayload()
	if len(d) == 0 {
		return nil
	}
	// Since we're eager, immediately call the next decoder.
	return next.Decode(d, p)
}
func (p *eagerPacket) initialDecode(dec Decoder) {
	defer p.recoverDecodeError()
	err := dec.Decode(p.data, p)
	if err != nil {
		p.addFinalDecodeError(err, nil)
	}
}
func (p *eagerPacket) LinkLayer() LinkLayer {
	return p.link
}
func (p *eagerPacket) NetworkLayer() NetworkLayer {
	return p.network
}
func (p *eagerPacket) TransportLayer() TransportLayer {
	return p.transport
}
func (p *eagerPacket) ApplicationLayer() ApplicationLayer {
	return p.application
}
func (p *eagerPacket) ErrorLayer() ErrorLayer {
	return p.failure
}
func (p *eagerPacket) Layers() []Layer {
	return p.layers
}
func (p *eagerPacket) Layer(t LayerType) Layer {
	for _, l := range p.layers {
		if l.LayerType() == t {
			return l
		}
	}
	return nil
}
func (p *eagerPacket) LayerClass(lc LayerClass) Layer {
	for _, l := range p.layers {
		if lc.Contains(l.LayerType()) {
			return l
		}
	}
	return nil
}
func (p *eagerPacket) String() string { return p.packetString() }
func (p *eagerPacket) Dump() string   { return p.packetDump() }

// lazyPacket does lazy decoding on its packet data.  On construction it does
// no initial decoding.  For each function call, it decodes only as many layers
// as are necessary to compute the return value for that function.
// lazyPacket implements Packet and PacketBuilder.
type lazyPacket struct {
	packet
	next Decoder
}

func (p *lazyPacket) NextDecoder(next Decoder) error {
	if next == nil {
		return errNilDecoder
	}
	p.next = next
	return nil
}
func (p *lazyPacket) decodeNextLayer() {
	if p.next == nil {
		return
	}
	d := p.data
	if p.last != nil {
		d = p.last.LayerPayload()
	}
	next := p.next
	p.next = nil
	// We've just set p.next to nil, so if we see we have no data, this should be
	// the final call we get to decodeNextLayer if we return here.
	if len(d) == 0 {
		return
	}
	defer p.recoverDecodeError()
	err := next.Decode(d, p)
	if err != nil {
		p.addFinalDecodeError(err, nil)
	}
}
func (p *lazyPacket) LinkLayer() LinkLayer {
	for p.link == nil && p.next != nil {
		p.decodeNextLayer()
	}
	return p.link
}
func (p *lazyPacket) NetworkLayer() NetworkLayer {
	for p.network == nil && p.next != nil {
		p.decodeNextLayer()
	}
	return p.network
}
func (p *lazyPacket) TransportLayer() TransportLayer {
	for p.transport == nil && p.next != nil {
		p.decodeNextLayer()
	}
	return p.transport
}
func (p *lazyPacket) ApplicationLayer() ApplicationLayer {
	for p.application == nil && p.next != nil {
		p.decodeNextLayer()
	}
	return p.application
}
func (p *lazyPacket) ErrorLayer() ErrorLayer {
	for p.failure == nil && p.next != nil {
		p.decodeNextLayer()
	}
	return p.failure
}
func (p *lazyPacket) Layers() []Layer {
	for p.next != nil {
		p.decodeNextLayer()
	}
	return p.layers
}
func (p *lazyPacket) Layer(t LayerType) Layer {
	for _, l := range p.layers {
		if l.LayerType() == t {
			return l
		}
	}
	numLayers := len(p.layers)
	for p.next != nil {
		p.decodeNextLayer()
		for _, l := range p.layers[numLayers:] {
			if l.LayerType() == t {
				return l
			}
		}
		numLayers = len(p.layers)
	}
	return nil
}
func (p *lazyPacket) LayerClass(lc LayerClass) Layer {
	for _, l := range p.layers {
		if lc.Contains(l.LayerType()) {
			return l
		}
	}
	numLayers := len(p.layers)
	for p.next != nil {
		p.decodeNextLayer()
		for _, l := range p.layers[numLayers:] {
			if lc.Contains(l.LayerType()) {
				return l
			}
		}
		numLayers = len(p.layers)
	}
	return nil
}
func (p *lazyPacket) String() string { p.Layers(); return p.packetString() }
func (p *lazyPacket) Dump() string   { p.Layers(); return p.packetDump() }

// DecodeOptions tells gopacket how to decode a packet.
type DecodeOptions struct {
	// Lazy decoding decodes the minimum number of layers needed to return data
	// for a packet at each function call.  Be careful using this with concurrent
	// packet processors, as each call to packet.* could mutate the packet, and
	// two concurrent function calls could interact poorly.
	Lazy bool
	// NoCopy decoding doesn't copy its input buffer into storage that's owned by
	// the packet.  If you can guarantee that the bytes underlying the slice
	// passed into NewPacket aren't going to be modified, this can be faster.  If
	// there's any chance that those bytes WILL be changed, this will invalidate
	// your packets.
	NoCopy bool
	// SkipDecodeRecovery skips over panic recovery during packet decoding.
	// Normally, when packets decode, if a panic occurs, that panic is captured
	// by a recover(), and a DecodeFailure layer is added to the packet detailing
	// the issue.  If this flag is set, panics are instead allowed to continue up
	// the stack.
	SkipDecodeRecovery bool
	// DecodeStreamsAsDatagrams enables routing of application-level layers in the TCP
	// decoder. If true, we should try to decode layers after TCP in single packets.
	// This is disabled by default because the reassembly package drives the decoding
	// of TCP payload data after reassembly.
	DecodeStreamsAsDatagrams bool
}

// Default decoding provides the safest (but slowest) method for decoding
// packets.  It eagerly processes all layers (so it's concurrency-safe) and it
// copies its input buffer upon creation of the packet (so the packet remains
// valid if the underlying slice is modified.  Both of these take time,
// though, so beware.  If you can guarantee that the packet will only be used
// by one goroutine at a time, set Lazy decoding.  If you can guarantee that
// the underlying slice won't change, set NoCopy decoding.
var Default = DecodeOptions{}

// Lazy is a DecodeOptions with just Lazy set.
var Lazy = DecodeOptions{Lazy: true}

// NoCopy is a DecodeOptions with just NoCopy set.
var NoCopy = DecodeOptions{NoCopy: true}

// DecodeStreamsAsDatagrams is a DecodeOptions with just DecodeStreamsAsDatagrams set.
var DecodeStreamsAsDatagrams = DecodeOptions{DecodeStreamsAsDatagrams: true}

// NewPacket creates a new Packet object from a set of bytes.  The
// firstLayerDecoder tells it how to interpret the first layer from the bytes,
// future layers will be generated from that first layer automatically.
func NewPacket(data []byte, firstLayerDecoder Decoder, options DecodeOptions) Packet {
	if !options.NoCopy {
		dataCopy := make([]byte, len(data))
		copy(dataCopy, data)
		data = dataCopy
	}
	if options.Lazy {
		p := &lazyPacket{
			packet: packet{data: data, decodeOptions: options},
			next:   firstLayerDecoder,
		}
		p.layers = p.initialLayers[:0]
		// Crazy craziness:
		// If the following return statemet is REMOVED, and Lazy is FALSE, then
		// eager packet processing becomes 17% FASTER.  No, there is no logical
		// explanation for this.  However, it's such a hacky micro-optimization that
		// we really can't rely on it.  It appears to have to do with the size the
		// compiler guesses for this function's stack space, since one symptom is
		// that with the return statement in place, we more than double calls to
		// runtime.morestack/runtime.lessstack.  We'll hope the compiler gets better
		// over time and we get this optimization for free.  Until then, we'll have
		// to live with slower packet processing.
		return p
	}
	p := &eagerPacket{
		packet: packet{data: data, decodeOptions: options},
	}
	p.layers = p.initialLayers[:0]
	p.initialDecode(firstLayerDecoder)
	return p
}

// PacketDataSource is an interface for some source of packet data.  Users may
// create their own implementations, or use the existing implementations in
// gopacket/pcap (libpcap, allows reading from live interfaces or from
// pcap files) or gopacket/pfring (PF_RING, allows reading from live
// interfaces).
type PacketDataSource interface {
	// ReadPacketData returns the next packet available from this data source.
	// It returns:
	//  data:  The bytes of an individual packet.
	//  ci:  Metadata about the capture
	//  err:  An error encountered while reading packet data.  If err != nil,
	//    then data/ci will be ignored.
	ReadPacketData() (data []byte, ci CaptureInfo, err error)
}

// ConcatFinitePacketDataSources returns a PacketDataSource that wraps a set
// of internal PacketDataSources, each of which will stop with io.EOF after
// reading a finite number of packets.  The returned PacketDataSource will
// return all packets from the first finite source, followed by all packets from
// the second, etc.  Once all finite sources have returned io.EOF, the returned
// source will as well.
func ConcatFinitePacketDataSources(pds ...PacketDataSource) PacketDataSource {
	c := concat(pds)
	return &c
}

type concat []PacketDataSource

func (c *concat) ReadPacketData() (data []byte, ci CaptureInfo, err error) {
	for len(*c) > 0 {
		data, ci, err = (*c)[0].ReadPacketData()
		if err == io.EOF {
			*c = (*c)[1:]
			continue
		}
		return
	}
	return nil, CaptureInfo{}, io.EOF
}

// ZeroCopyPacketDataSource is an interface to pull packet data from sources
// that allow data to be returned without copying to a user-controlled buffer.
// It's very similar to PacketDataSource, except that the caller must be more
// careful in how the returned buffer is handled.
type ZeroCopyPacketDataSource interface {
	// ZeroCopyReadPacketData returns the next packet available from this data source.
	// It returns:
	//  data:  The bytes of an individual packet.  Unlike with
	//    PacketDataSource's ReadPacketData, the slice returned here points
	//    to a buffer owned by the data source.  In particular, the bytes in
	//    this buffer may be changed by future calls to
	//    ZeroCopyReadPacketData.  Do not use the returned buffer after
	//    subsequent ZeroCopyReadPacketData calls.
	//  ci:  Metadata about the capture
	//  err:  An error encountered while reading packet data.  If err != nil,
	//    then data/ci will be ignored.
	ZeroCopyReadPacketData() (data []byte, ci CaptureInfo, err error)
}

// PacketSource reads in packets from a PacketDataSource, decodes them, and
// returns them.
//
// There are currently two different methods for reading packets in through
// a PacketSource:
//
// Reading With Packets Function
//
// This method is the most convenient and easiest to code, but lacks
// flexibility.  Packets returns a 'chan Packet', then asynchronously writes
// packets into that channel.  Packets uses a blocking channel, and closes
// it if an io.EOF is returned by the underlying PacketDataSource.  All other
// PacketDataSource errors are ignored and discarded.
//  for packet := range packetSource.Packets() {
//    ...
//  }
//
// Reading With NextPacket Function
//
// This method is the most flexible, and exposes errors that may be
// encountered by the underlying PacketDataSource.  It's also the fastest
// in a tight loop, since it doesn't have the overhead of a channel
// read/write.  However, it requires the user to handle errors, most
// importantly the io.EOF error in cases where packets are being read from
// a file.
//  for {
//    packet, err := packetSource.NextPacket()
//    if err == io.EOF {
//      break
//    } else if err != nil {
//      log.Println("Error:", err)
//      continue
//    }
//    handlePacket(packet)  // Do something with each packet.
//  }
type PacketSource struct {
	source  PacketDataSource
	decoder Decoder
	// DecodeOptions is the set of options to use for decoding each piece
	// of packet data.  This can/should be changed by the user to reflect the
	// way packets should be decoded.
	DecodeOptions
	c chan Packet
}

// NewPacketSource creates a packet data source.
func NewPacketSource(source PacketDataSource, decoder Decoder) *PacketSource {
	return &PacketSource{
		source:  source,
		decoder: decoder,
	}
}

// NextPacket returns the next decoded packet from the PacketSource.  On error,
// it returns a nil packet and a non-nil error.
func (p *PacketSource) NextPacket() (Packet, error) {
	data, ci, err := p.source.ReadPacketData()
	if err != nil {
		return nil, err
	}
	packet := NewPacket(data, p.decoder, p.DecodeOptions)
	m := packet.Metadata()
	m.CaptureInfo = ci
	m.Truncated = m.Truncated || ci.CaptureLength < ci.Length
	return packet, nil
}

// packetsToChannel reads in all packets from the packet source and sends them
// to the given channel. This routine terminates when a non-temporary error
// is returned by NextPacket().
func (p *PacketSource) packetsToChannel() {
	defer close(p.c)
	for {
		packet, err := p.NextPacket()
		if err == nil {
			p.c <- packet
			continue
		}

		// Immediately retry for temporary network errors
		if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
			continue
		}

		// Immediately retry for EAGAIN
		if err == syscall.EAGAIN {
			continue
		}

		// Immediately break for known unrecoverable errors
		if err == io.EOF || err == io.ErrUnexpectedEOF ||
			err == io.ErrNoProgress || err == io.ErrClosedPipe || err == io.ErrShortBuffer ||
			err == syscall.EBADF ||
			strings.Contains(err.Error(), "use of closed file") {
			break
		}

		// Sleep briefly and try again
		time.Sleep(time.Millisecond * time.Duration(5))
	}
}

// Packets returns a channel of packets, allowing easy iterating over
// packets.  Packets will be asynchronously read in from the underlying
// PacketDataSource and written to the returned channel.  If the underlying
// PacketDataSource returns an io.EOF error, the channel will be closed.
// If any other error is encountered, it is ignored.
//
//  for packet := range packetSource.Packets() {
//    handlePacket(packet)  // Do something with each packet.
//  }
//
// If called more than once, returns the same channel.
func (p *PacketSource) Packets() chan Packet {
	if p.c == nil {
		p.c = make(chan Packet, 1000)
		go p.packetsToChannel()
	}
	return p.c
}
