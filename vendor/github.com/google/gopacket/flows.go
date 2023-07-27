// Copyright 2012 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package gopacket

import (
	"bytes"
	"fmt"
	"strconv"
)

// MaxEndpointSize determines the maximum size in bytes of an endpoint address.
//
// Endpoints/Flows have a problem:  They need to be hashable.  Therefore, they
// can't use a byte slice.  The two obvious choices are to use a string or a
// byte array.  Strings work great, but string creation requires memory
// allocation, which can be slow.  Arrays work great, but have a fixed size.  We
// originally used the former, now we've switched to the latter.  Use of a fixed
// byte-array doubles the speed of constructing a flow (due to not needing to
// allocate).  This is a huge increase... too much for us to pass up.
//
// The end result of this, though, is that an endpoint/flow can't be created
// using more than MaxEndpointSize bytes per address.
const MaxEndpointSize = 16

// Endpoint is the set of bytes used to address packets at various layers.
// See LinkLayer, NetworkLayer, and TransportLayer specifications.
// Endpoints are usable as map keys.
type Endpoint struct {
	typ EndpointType
	len int
	raw [MaxEndpointSize]byte
}

// EndpointType returns the endpoint type associated with this endpoint.
func (a Endpoint) EndpointType() EndpointType { return a.typ }

// Raw returns the raw bytes of this endpoint.  These aren't human-readable
// most of the time, but they are faster than calling String.
func (a Endpoint) Raw() []byte { return a.raw[:a.len] }

// LessThan provides a stable ordering for all endpoints.  It sorts first based
// on the EndpointType of an endpoint, then based on the raw bytes of that
// endpoint.
//
// For some endpoints, the actual comparison may not make sense, however this
// ordering does provide useful information for most Endpoint types.
// Ordering is based first on endpoint type, then on raw endpoint bytes.
// Endpoint bytes are sorted lexicographically.
func (a Endpoint) LessThan(b Endpoint) bool {
	return a.typ < b.typ || (a.typ == b.typ && bytes.Compare(a.raw[:a.len], b.raw[:b.len]) < 0)
}

// fnvHash is used by our FastHash functions, and implements the FNV hash
// created by Glenn Fowler, Landon Curt Noll, and Phong Vo.
// See http://isthe.com/chongo/tech/comp/fnv/.
func fnvHash(s []byte) (h uint64) {
	h = fnvBasis
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnvPrime
	}
	return
}

const fnvBasis = 14695981039346656037
const fnvPrime = 1099511628211

// FastHash provides a quick hashing function for an endpoint, useful if you'd
// like to split up endpoints by modulos or other load-balancing techniques.
// It uses a variant of Fowler-Noll-Vo hashing.
//
// The output of FastHash is not guaranteed to remain the same through future
// code revisions, so should not be used to key values in persistent storage.
func (a Endpoint) FastHash() (h uint64) {
	h = fnvHash(a.raw[:a.len])
	h ^= uint64(a.typ)
	h *= fnvPrime
	return
}

// NewEndpoint creates a new Endpoint object.
//
// The size of raw must be less than MaxEndpointSize, otherwise this function
// will panic.
func NewEndpoint(typ EndpointType, raw []byte) (e Endpoint) {
	e.len = len(raw)
	if e.len > MaxEndpointSize {
		panic("raw byte length greater than MaxEndpointSize")
	}
	e.typ = typ
	copy(e.raw[:], raw)
	return
}

// EndpointTypeMetadata is used to register a new endpoint type.
type EndpointTypeMetadata struct {
	// Name is the string returned by an EndpointType's String function.
	Name string
	// Formatter is called from an Endpoint's String function to format the raw
	// bytes in an Endpoint into a human-readable string.
	Formatter func([]byte) string
}

// EndpointType is the type of a gopacket Endpoint.  This type determines how
// the bytes stored in the endpoint should be interpreted.
type EndpointType int64

var endpointTypes = map[EndpointType]EndpointTypeMetadata{}

// RegisterEndpointType creates a new EndpointType and registers it globally.
// It MUST be passed a unique number, or it will panic.  Numbers 0-999 are
// reserved for gopacket's use.
func RegisterEndpointType(num int, meta EndpointTypeMetadata) EndpointType {
	t := EndpointType(num)
	if _, ok := endpointTypes[t]; ok {
		panic("Endpoint type number already in use")
	}
	endpointTypes[t] = meta
	return t
}

func (e EndpointType) String() string {
	if t, ok := endpointTypes[e]; ok {
		return t.Name
	}
	return strconv.Itoa(int(e))
}

func (a Endpoint) String() string {
	if t, ok := endpointTypes[a.typ]; ok && t.Formatter != nil {
		return t.Formatter(a.raw[:a.len])
	}
	return fmt.Sprintf("%v:%v", a.typ, a.raw)
}

// Flow represents the direction of traffic for a packet layer, as a source and destination Endpoint.
// Flows are usable as map keys.
type Flow struct {
	typ        EndpointType
	slen, dlen int
	src, dst   [MaxEndpointSize]byte
}

// FlowFromEndpoints creates a new flow by pasting together two endpoints.
// The endpoints must have the same EndpointType, or this function will return
// an error.
func FlowFromEndpoints(src, dst Endpoint) (_ Flow, err error) {
	if src.typ != dst.typ {
		err = fmt.Errorf("Mismatched endpoint types: %v->%v", src.typ, dst.typ)
		return
	}
	return Flow{src.typ, src.len, dst.len, src.raw, dst.raw}, nil
}

// FastHash provides a quick hashing function for a flow, useful if you'd
// like to split up flows by modulos or other load-balancing techniques.
// It uses a variant of Fowler-Noll-Vo hashing, and is guaranteed to collide
// with its reverse flow.  IE: the flow A->B will have the same hash as the flow
// B->A.
//
// The output of FastHash is not guaranteed to remain the same through future
// code revisions, so should not be used to key values in persistent storage.
func (f Flow) FastHash() (h uint64) {
	// This combination must be commutative.  We don't use ^, since that would
	// give the same hash for all A->A flows.
	h = fnvHash(f.src[:f.slen]) + fnvHash(f.dst[:f.dlen])
	h ^= uint64(f.typ)
	h *= fnvPrime
	return
}

// String returns a human-readable representation of this flow, in the form
// "Src->Dst"
func (f Flow) String() string {
	s, d := f.Endpoints()
	return fmt.Sprintf("%v->%v", s, d)
}

// EndpointType returns the EndpointType for this Flow.
func (f Flow) EndpointType() EndpointType {
	return f.typ
}

// Endpoints returns the two Endpoints for this flow.
func (f Flow) Endpoints() (src, dst Endpoint) {
	return Endpoint{f.typ, f.slen, f.src}, Endpoint{f.typ, f.dlen, f.dst}
}

// Src returns the source Endpoint for this flow.
func (f Flow) Src() (src Endpoint) {
	src, _ = f.Endpoints()
	return
}

// Dst returns the destination Endpoint for this flow.
func (f Flow) Dst() (dst Endpoint) {
	_, dst = f.Endpoints()
	return
}

// Reverse returns a new flow with endpoints reversed.
func (f Flow) Reverse() Flow {
	return Flow{f.typ, f.dlen, f.slen, f.dst, f.src}
}

// NewFlow creates a new flow.
//
// src and dst must have length <= MaxEndpointSize, otherwise NewFlow will
// panic.
func NewFlow(t EndpointType, src, dst []byte) (f Flow) {
	f.slen = len(src)
	f.dlen = len(dst)
	if f.slen > MaxEndpointSize || f.dlen > MaxEndpointSize {
		panic("flow raw byte length greater than MaxEndpointSize")
	}
	f.typ = t
	copy(f.src[:], src)
	copy(f.dst[:], dst)
	return
}

// EndpointInvalid is an endpoint type used for invalid endpoints, IE endpoints
// that are specified incorrectly during creation.
var EndpointInvalid = RegisterEndpointType(0, EndpointTypeMetadata{Name: "invalid", Formatter: func(b []byte) string {
	return fmt.Sprintf("%v", b)
}})

// InvalidEndpoint is a singleton Endpoint of type EndpointInvalid.
var InvalidEndpoint = NewEndpoint(EndpointInvalid, nil)

// InvalidFlow is a singleton Flow of type EndpointInvalid.
var InvalidFlow = NewFlow(EndpointInvalid, nil, nil)
