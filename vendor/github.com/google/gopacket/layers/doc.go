// Copyright 2012 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

/*
Package layers provides decoding layers for many common protocols.

The layers package contains decode implementations for a number of different
types of packet layers.  Users of gopacket will almost always want to also use
layers to actually decode packet data into useful pieces. To see the set of
protocols that gopacket/layers is currently able to decode,
look at the set of LayerTypes defined in the Variables sections. The
layers package also defines endpoints for many of the common packet layers
that have source/destination addresses associated with them, for example IPv4/6
(IPs) and TCP/UDP (ports).
Finally, layers contains a number of useful enumerations (IPProtocol,
EthernetType, LinkType, PPPType, etc...).  Many of these implement the
gopacket.Decoder interface, so they can be passed into gopacket as decoders.

Most common protocol layers are named using acronyms or other industry-common
names (IPv4, TCP, PPP).  Some of the less common ones have their names expanded
(CiscoDiscoveryProtocol).
For certain protocols, sub-parts of the protocol are split out into their own
layers (SCTP, for example).  This is done mostly in cases where portions of the
protocol may fulfill the capabilities of interesting layers (SCTPData implements
ApplicationLayer, while base SCTP implements TransportLayer), or possibly
because splitting a protocol into a few layers makes decoding easier.

This package is meant to be used with its parent,
http://github.com/google/gopacket.

Port Types

Instead of using raw uint16 or uint8 values for ports, we use a different port
type for every protocol, for example TCPPort and UDPPort.  This allows us to
override string behavior for each port, which we do by setting up port name
maps (TCPPortNames, UDPPortNames, etc...).  Well-known ports are annotated with
their protocol names, and their String function displays these names:

  p := TCPPort(80)
  fmt.Printf("Number: %d  String: %v", p, p)
  // Prints: "Number: 80  String: 80(http)"

Modifying Decode Behavior

layers links together decoding through its enumerations.  For example, after
decoding layer type Ethernet, it uses Ethernet.EthernetType as its next decoder.
All enumerations that act as decoders, like EthernetType, can be modified by
users depending on their preferences.  For example, if you have a spiffy new
IPv4 decoder that works way better than the one built into layers, you can do
this:

 var mySpiffyIPv4Decoder gopacket.Decoder = ...
 layers.EthernetTypeMetadata[EthernetTypeIPv4].DecodeWith = mySpiffyIPv4Decoder

This will make all future ethernet packets use your new decoder to decode IPv4
packets, instead of the built-in decoder used by gopacket.
*/
package layers
