// Copyright 2012 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

/*
Package gopacket provides packet decoding for the Go language.

gopacket contains many sub-packages with additional functionality you may find
useful, including:

 * layers: You'll probably use this every time.  This contains of the logic
     built into gopacket for decoding packet protocols.  Note that all example
     code below assumes that you have imported both gopacket and
     gopacket/layers.
 * pcap: C bindings to use libpcap to read packets off the wire.
 * pfring: C bindings to use PF_RING to read packets off the wire.
 * afpacket: C bindings for Linux's AF_PACKET to read packets off the wire.
 * tcpassembly: TCP stream reassembly

Also, if you're looking to dive right into code, see the examples subdirectory
for numerous simple binaries built using gopacket libraries.

Minimum go version required is 1.5 except for pcapgo/EthernetHandle, afpacket,
and bsdbpf which need at least 1.7 due to x/sys/unix dependencies.

Basic Usage

gopacket takes in packet data as a []byte and decodes it into a packet with
a non-zero number of "layers".  Each layer corresponds to a protocol
within the bytes.  Once a packet has been decoded, the layers of the packet
can be requested from the packet.

 // Decode a packet
 packet := gopacket.NewPacket(myPacketData, layers.LayerTypeEthernet, gopacket.Default)
 // Get the TCP layer from this packet
 if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
   fmt.Println("This is a TCP packet!")
   // Get actual TCP data from this layer
   tcp, _ := tcpLayer.(*layers.TCP)
   fmt.Printf("From src port %d to dst port %d\n", tcp.SrcPort, tcp.DstPort)
 }
 // Iterate over all layers, printing out each layer type
 for _, layer := range packet.Layers() {
   fmt.Println("PACKET LAYER:", layer.LayerType())
 }

Packets can be decoded from a number of starting points.  Many of our base
types implement Decoder, which allow us to decode packets for which
we don't have full data.

 // Decode an ethernet packet
 ethP := gopacket.NewPacket(p1, layers.LayerTypeEthernet, gopacket.Default)
 // Decode an IPv6 header and everything it contains
 ipP := gopacket.NewPacket(p2, layers.LayerTypeIPv6, gopacket.Default)
 // Decode a TCP header and its payload
 tcpP := gopacket.NewPacket(p3, layers.LayerTypeTCP, gopacket.Default)


Reading Packets From A Source

Most of the time, you won't just have a []byte of packet data lying around.
Instead, you'll want to read packets in from somewhere (file, interface, etc)
and process them.  To do that, you'll want to build a PacketSource.

First, you'll need to construct an object that implements the PacketDataSource
interface.  There are implementations of this interface bundled with gopacket
in the gopacket/pcap and gopacket/pfring subpackages... see their documentation
for more information on their usage.  Once you have a PacketDataSource, you can
pass it into NewPacketSource, along with a Decoder of your choice, to create
a PacketSource.

Once you have a PacketSource, you can read packets from it in multiple ways.
See the docs for PacketSource for more details.  The easiest method is the
Packets function, which returns a channel, then asynchronously writes new
packets into that channel, closing the channel if the packetSource hits an
end-of-file.

  packetSource := ...  // construct using pcap or pfring
  for packet := range packetSource.Packets() {
    handlePacket(packet)  // do something with each packet
  }

You can change the decoding options of the packetSource by setting fields in
packetSource.DecodeOptions... see the following sections for more details.


Lazy Decoding

gopacket optionally decodes packet data lazily, meaning it
only decodes a packet layer when it needs to handle a function call.

 // Create a packet, but don't actually decode anything yet
 packet := gopacket.NewPacket(myPacketData, layers.LayerTypeEthernet, gopacket.Lazy)
 // Now, decode the packet up to the first IPv4 layer found but no further.
 // If no IPv4 layer was found, the whole packet will be decoded looking for
 // it.
 ip4 := packet.Layer(layers.LayerTypeIPv4)
 // Decode all layers and return them.  The layers up to the first IPv4 layer
 // are already decoded, and will not require decoding a second time.
 layers := packet.Layers()

Lazily-decoded packets are not concurrency-safe.  Since layers have not all been
decoded, each call to Layer() or Layers() has the potential to mutate the packet
in order to decode the next layer.  If a packet is used
in multiple goroutines concurrently, don't use gopacket.Lazy.  Then gopacket
will decode the packet fully, and all future function calls won't mutate the
object.


NoCopy Decoding

By default, gopacket will copy the slice passed to NewPacket and store the
copy within the packet, so future mutations to the bytes underlying the slice
don't affect the packet and its layers.  If you can guarantee that the
underlying slice bytes won't be changed, you can use NoCopy to tell
gopacket.NewPacket, and it'll use the passed-in slice itself.

 // This channel returns new byte slices, each of which points to a new
 // memory location that's guaranteed immutable for the duration of the
 // packet.
 for data := range myByteSliceChannel {
   p := gopacket.NewPacket(data, layers.LayerTypeEthernet, gopacket.NoCopy)
   doSomethingWithPacket(p)
 }

The fastest method of decoding is to use both Lazy and NoCopy, but note from
the many caveats above that for some implementations either or both may be
dangerous.


Pointers To Known Layers

During decoding, certain layers are stored in the packet as well-known
layer types.  For example, IPv4 and IPv6 are both considered NetworkLayer
layers, while TCP and UDP are both TransportLayer layers.  We support 4
layers, corresponding to the 4 layers of the TCP/IP layering scheme (roughly
anagalous to layers 2, 3, 4, and 7 of the OSI model).  To access these,
you can use the packet.LinkLayer, packet.NetworkLayer,
packet.TransportLayer, and packet.ApplicationLayer functions.  Each of
these functions returns a corresponding interface
(gopacket.{Link,Network,Transport,Application}Layer).  The first three
provide methods for getting src/dst addresses for that particular layer,
while the final layer provides a Payload function to get payload data.
This is helpful, for example, to get payloads for all packets regardless
of their underlying data type:

 // Get packets from some source
 for packet := range someSource {
   if app := packet.ApplicationLayer(); app != nil {
     if strings.Contains(string(app.Payload()), "magic string") {
       fmt.Println("Found magic string in a packet!")
     }
   }
 }

A particularly useful layer is ErrorLayer, which is set whenever there's
an error parsing part of the packet.

 packet := gopacket.NewPacket(myPacketData, layers.LayerTypeEthernet, gopacket.Default)
 if err := packet.ErrorLayer(); err != nil {
   fmt.Println("Error decoding some part of the packet:", err)
 }

Note that we don't return an error from NewPacket because we may have decoded
a number of layers successfully before running into our erroneous layer.  You
may still be able to get your Ethernet and IPv4 layers correctly, even if
your TCP layer is malformed.


Flow And Endpoint

gopacket has two useful objects, Flow and Endpoint, for communicating in a protocol
independent manner the fact that a packet is coming from A and going to B.
The general layer types LinkLayer, NetworkLayer, and TransportLayer all provide
methods for extracting their flow information, without worrying about the type
of the underlying Layer.

A Flow is a simple object made up of a set of two Endpoints, one source and one
destination.  It details the sender and receiver of the Layer of the Packet.

An Endpoint is a hashable representation of a source or destination.  For
example, for LayerTypeIPv4, an Endpoint contains the IP address bytes for a v4
IP packet.  A Flow can be broken into Endpoints, and Endpoints can be combined
into Flows:

 packet := gopacket.NewPacket(myPacketData, layers.LayerTypeEthernet, gopacket.Lazy)
 netFlow := packet.NetworkLayer().NetworkFlow()
 src, dst := netFlow.Endpoints()
 reverseFlow := gopacket.NewFlow(dst, src)

Both Endpoint and Flow objects can be used as map keys, and the equality
operator can compare them, so you can easily group together all packets
based on endpoint criteria:

 flows := map[gopacket.Endpoint]chan gopacket.Packet
 packet := gopacket.NewPacket(myPacketData, layers.LayerTypeEthernet, gopacket.Lazy)
 // Send all TCP packets to channels based on their destination port.
 if tcp := packet.Layer(layers.LayerTypeTCP); tcp != nil {
   flows[tcp.TransportFlow().Dst()] <- packet
 }
 // Look for all packets with the same source and destination network address
 if net := packet.NetworkLayer(); net != nil {
   src, dst := net.NetworkFlow().Endpoints()
   if src == dst {
     fmt.Println("Fishy packet has same network source and dst: %s", src)
   }
 }
 // Find all packets coming from UDP port 1000 to UDP port 500
 interestingFlow := gopacket.FlowFromEndpoints(layers.NewUDPPortEndpoint(1000), layers.NewUDPPortEndpoint(500))
 if t := packet.NetworkLayer(); t != nil && t.TransportFlow() == interestingFlow {
   fmt.Println("Found that UDP flow I was looking for!")
 }

For load-balancing purposes, both Flow and Endpoint have FastHash() functions,
which provide quick, non-cryptographic hashes of their contents.  Of particular
importance is the fact that Flow FastHash() is symmetric: A->B will have the same
hash as B->A.  An example usage could be:

 channels := [8]chan gopacket.Packet
 for i := 0; i < 8; i++ {
   channels[i] = make(chan gopacket.Packet)
   go packetHandler(channels[i])
 }
 for packet := range getPackets() {
   if net := packet.NetworkLayer(); net != nil {
     channels[int(net.NetworkFlow().FastHash()) & 0x7] <- packet
   }
 }

This allows us to split up a packet stream while still making sure that each
stream sees all packets for a flow (and its bidirectional opposite).


Implementing Your Own Decoder

If your network has some strange encapsulation, you can implement your own
decoder.  In this example, we handle Ethernet packets which are encapsulated
in a 4-byte header.

 // Create a layer type, should be unique and high, so it doesn't conflict,
 // giving it a name and a decoder to use.
 var MyLayerType = gopacket.RegisterLayerType(12345, gopacket.LayerTypeMetadata{Name: "MyLayerType", Decoder: gopacket.DecodeFunc(decodeMyLayer)})

 // Implement my layer
 type MyLayer struct {
   StrangeHeader []byte
   payload []byte
 }
 func (m MyLayer) LayerType() gopacket.LayerType { return MyLayerType }
 func (m MyLayer) LayerContents() []byte { return m.StrangeHeader }
 func (m MyLayer) LayerPayload() []byte { return m.payload }

 // Now implement a decoder... this one strips off the first 4 bytes of the
 // packet.
 func decodeMyLayer(data []byte, p gopacket.PacketBuilder) error {
   // Create my layer
   p.AddLayer(&MyLayer{data[:4], data[4:]})
   // Determine how to handle the rest of the packet
   return p.NextDecoder(layers.LayerTypeEthernet)
 }

 // Finally, decode your packets:
 p := gopacket.NewPacket(data, MyLayerType, gopacket.Lazy)

See the docs for Decoder and PacketBuilder for more details on how coding
decoders works, or look at RegisterLayerType and RegisterEndpointType to see how
to add layer/endpoint types to gopacket.


Fast Decoding With DecodingLayerParser

TLDR:  DecodingLayerParser takes about 10% of the time as NewPacket to decode
packet data, but only for known packet stacks.

Basic decoding using gopacket.NewPacket or PacketSource.Packets is somewhat slow
due to its need to allocate a new packet and every respective layer.  It's very
versatile and can handle all known layer types, but sometimes you really only
care about a specific set of layers regardless, so that versatility is wasted.

DecodingLayerParser avoids memory allocation altogether by decoding packet
layers directly into preallocated objects, which you can then reference to get
the packet's information.  A quick example:

 func main() {
   var eth layers.Ethernet
   var ip4 layers.IPv4
   var ip6 layers.IPv6
   var tcp layers.TCP
   parser := gopacket.NewDecodingLayerParser(layers.LayerTypeEthernet, &eth, &ip4, &ip6, &tcp)
   decoded := []gopacket.LayerType{}
   for packetData := range somehowGetPacketData() {
     if err := parser.DecodeLayers(packetData, &decoded); err != nil {
       fmt.Fprintf(os.Stderr, "Could not decode layers: %v\n", err)
       continue
     }
     for _, layerType := range decoded {
       switch layerType {
         case layers.LayerTypeIPv6:
           fmt.Println("    IP6 ", ip6.SrcIP, ip6.DstIP)
         case layers.LayerTypeIPv4:
           fmt.Println("    IP4 ", ip4.SrcIP, ip4.DstIP)
       }
     }
   }
 }

The important thing to note here is that the parser is modifying the passed in
layers (eth, ip4, ip6, tcp) instead of allocating new ones, thus greatly
speeding up the decoding process.  It's even branching based on layer type...
it'll handle an (eth, ip4, tcp) or (eth, ip6, tcp) stack.  However, it won't
handle any other type... since no other decoders were passed in, an (eth, ip4,
udp) stack will stop decoding after ip4, and only pass back [LayerTypeEthernet,
LayerTypeIPv4] through the 'decoded' slice (along with an error saying it can't
decode a UDP packet).

Unfortunately, not all layers can be used by DecodingLayerParser... only those
implementing the DecodingLayer interface are usable.  Also, it's possible to
create DecodingLayers that are not themselves Layers... see
layers.IPv6ExtensionSkipper for an example of this.

Faster And Customized Decoding with DecodingLayerContainer

By default, DecodingLayerParser uses native map to store and search for a layer
to decode. Though being versatile, in some cases this solution may be not so
optimal. For example, if you have only few layers faster operations may be
provided by sparse array indexing or linear array scan.

To accomodate these scenarios, DecodingLayerContainer interface is introduced
along with its implementations: DecodingLayerSparse, DecodingLayerArray and
DecodingLayerMap. You can specify a container implementation to
DecodingLayerParser with SetDecodingLayerContainer method. Example:

 dlp := gopacket.NewDecodingLayerParser(LayerTypeEthernet)
 dlp.SetDecodingLayerContainer(gopacket.DecodingLayerSparse(nil))
 var eth layers.Ethernet
 dlp.AddDecodingLayer(&eth)
 // ... add layers and use DecodingLayerParser as usual...

To skip one level of indirection (though sacrificing some capabilities) you may
also use DecodingLayerContainer as a decoding tool as it is. In this case you have to
handle unknown layer types and layer panics by yourself. Example:

 func main() {
   var eth layers.Ethernet
   var ip4 layers.IPv4
   var ip6 layers.IPv6
   var tcp layers.TCP
   dlc := gopacket.DecodingLayerContainer(gopacket.DecodingLayerArray(nil))
   dlc = dlc.Put(&eth)
   dlc = dlc.Put(&ip4)
   dlc = dlc.Put(&ip6)
   dlc = dlc.Put(&tcp)
   // you may specify some meaningful DecodeFeedback
   decoder := dlc.LayersDecoder(LayerTypeEthernet, gopacket.NilDecodeFeedback)
   decoded := make([]gopacket.LayerType, 0, 20)
   for packetData := range somehowGetPacketData() {
     lt, err := decoder(packetData, &decoded)
     if err != nil {
       fmt.Fprintf(os.Stderr, "Could not decode layers: %v\n", err)
       continue
     }
     if lt != gopacket.LayerTypeZero {
       fmt.Fprintf(os.Stderr, "unknown layer type: %v\n", lt)
       continue
     }
     for _, layerType := range decoded {
       // examine decoded layertypes just as already shown above
     }
   }
 }

DecodingLayerSparse is the fastest but most effective when LayerType values
that layers in use can decode are not large because otherwise that would lead
to bigger memory footprint. DecodingLayerArray is very compact and primarily
usable if the number of decoding layers is not big (up to ~10-15, but please do
your own benchmarks). DecodingLayerMap is the most versatile one and used by
DecodingLayerParser by default. Please refer to tests and benchmarks in layers
subpackage to further examine usage examples and performance measurements.

You may also choose to implement your own DecodingLayerContainer if you want to
make use of your own internal packet decoding logic.

Creating Packet Data

As well as offering the ability to decode packet data, gopacket will allow you
to create packets from scratch, as well.  A number of gopacket layers implement
the SerializableLayer interface; these layers can be serialized to a []byte in
the following manner:

  ip := &layers.IPv4{
    SrcIP: net.IP{1, 2, 3, 4},
    DstIP: net.IP{5, 6, 7, 8},
    // etc...
  }
  buf := gopacket.NewSerializeBuffer()
  opts := gopacket.SerializeOptions{}  // See SerializeOptions for more details.
  err := ip.SerializeTo(buf, opts)
  if err != nil { panic(err) }
  fmt.Println(buf.Bytes())  // prints out a byte slice containing the serialized IPv4 layer.

SerializeTo PREPENDS the given layer onto the SerializeBuffer, and they treat
the current buffer's Bytes() slice as the payload of the serializing layer.
Therefore, you can serialize an entire packet by serializing a set of layers in
reverse order (Payload, then TCP, then IP, then Ethernet, for example).  The
SerializeBuffer's SerializeLayers function is a helper that does exactly that.

To generate a (empty and useless, because no fields are set)
Ethernet(IPv4(TCP(Payload))) packet, for example, you can run:

  buf := gopacket.NewSerializeBuffer()
  opts := gopacket.SerializeOptions{}
  gopacket.SerializeLayers(buf, opts,
    &layers.Ethernet{},
    &layers.IPv4{},
    &layers.TCP{},
    gopacket.Payload([]byte{1, 2, 3, 4}))
  packetData := buf.Bytes()

A Final Note

If you use gopacket, you'll almost definitely want to make sure gopacket/layers
is imported, since when imported it sets all the LayerType variables and fills
in a lot of interesting variables/maps (DecodersByLayerName, etc).  Therefore,
it's recommended that even if you don't use any layers functions directly, you still import with:

  import (
    _ "github.com/google/gopacket/layers"
  )
*/
package gopacket
