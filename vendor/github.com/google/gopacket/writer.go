// Copyright 2012 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package gopacket

import (
	"fmt"
)

// SerializableLayer allows its implementations to be written out as a set of bytes,
// so those bytes may be sent on the wire or otherwise used by the caller.
// SerializableLayer is implemented by certain Layer types, and can be encoded to
// bytes using the LayerWriter object.
type SerializableLayer interface {
	// SerializeTo writes this layer to a slice, growing that slice if necessary
	// to make it fit the layer's data.
	//  Args:
	//   b:  SerializeBuffer to write this layer on to.  When called, b.Bytes()
	//     is the payload this layer should wrap, if any.  Note that this
	//     layer can either prepend itself (common), append itself
	//     (uncommon), or both (sometimes padding or footers are required at
	//     the end of packet data). It's also possible (though probably very
	//     rarely needed) to overwrite any bytes in the current payload.
	//     After this call, b.Bytes() should return the byte encoding of
	//     this layer wrapping the original b.Bytes() payload.
	//   opts:  options to use while writing out data.
	//  Returns:
	//   error if a problem was encountered during encoding.  If an error is
	//   returned, the bytes in data should be considered invalidated, and
	//   not used.
	//
	// SerializeTo calls SHOULD entirely ignore LayerContents and
	// LayerPayload.  It just serializes based on struct fields, neither
	// modifying nor using contents/payload.
	SerializeTo(b SerializeBuffer, opts SerializeOptions) error
	// LayerType returns the type of the layer that is being serialized to the buffer
	LayerType() LayerType
}

// SerializeOptions provides options for behaviors that SerializableLayers may want to
// implement.
type SerializeOptions struct {
	// FixLengths determines whether, during serialization, layers should fix
	// the values for any length field that depends on the payload.
	FixLengths bool
	// ComputeChecksums determines whether, during serialization, layers
	// should recompute checksums based on their payloads.
	ComputeChecksums bool
}

// SerializeBuffer is a helper used by gopacket for writing out packet layers.
// SerializeBuffer starts off as an empty []byte.  Subsequent calls to PrependBytes
// return byte slices before the current Bytes(), AppendBytes returns byte
// slices after.
//
// Byte slices returned by PrependBytes/AppendBytes are NOT zero'd out, so if
// you want to make sure they're all zeros, set them as such.
//
// SerializeBuffer is specifically designed to handle packet writing, where unlike
// with normal writes it's easier to start writing at the inner-most layer and
// work out, meaning that we often need to prepend bytes.  This runs counter to
// typical writes to byte slices using append(), where we only write at the end
// of the buffer.
//
// It can be reused via Clear.  Note, however, that a Clear call will invalidate the
// byte slices returned by any previous Bytes() call (the same buffer is
// reused).
//
//  1) Reusing a write buffer is generally much faster than creating a new one,
//     and with the default implementation it avoids additional memory allocations.
//  2) If a byte slice from a previous Bytes() call will continue to be used,
//     it's better to create a new SerializeBuffer.
//
// The Clear method is specifically designed to minimize memory allocations for
// similar later workloads on the SerializeBuffer.  IE: if you make a set of
// Prepend/Append calls, then clear, then make the same calls with the same
// sizes, the second round (and all future similar rounds) shouldn't allocate
// any new memory.
type SerializeBuffer interface {
	// Bytes returns the contiguous set of bytes collected so far by Prepend/Append
	// calls.  The slice returned by Bytes will be modified by future Clear calls,
	// so if you're planning on clearing this SerializeBuffer, you may want to copy
	// Bytes somewhere safe first.
	Bytes() []byte
	// PrependBytes returns a set of bytes which prepends the current bytes in this
	// buffer.  These bytes start in an indeterminate state, so they should be
	// overwritten by the caller.  The caller must only call PrependBytes if they
	// know they're going to immediately overwrite all bytes returned.
	PrependBytes(num int) ([]byte, error)
	// AppendBytes returns a set of bytes which appends the current bytes in this
	// buffer.  These bytes start in an indeterminate state, so they should be
	// overwritten by the caller.  The caller must only call AppendBytes if they
	// know they're going to immediately overwrite all bytes returned.
	AppendBytes(num int) ([]byte, error)
	// Clear resets the SerializeBuffer to a new, empty buffer.  After a call to clear,
	// the byte slice returned by any previous call to Bytes() for this buffer
	// should be considered invalidated.
	Clear() error
	// Layers returns all the Layers that have been successfully serialized into this buffer
	// already.
	Layers() []LayerType
	// PushLayer adds the current Layer to the list of Layers that have been serialized
	// into this buffer.
	PushLayer(LayerType)
}

type serializeBuffer struct {
	data                []byte
	start               int
	prepended, appended int
	layers              []LayerType
}

// NewSerializeBuffer creates a new instance of the default implementation of
// the SerializeBuffer interface.
func NewSerializeBuffer() SerializeBuffer {
	return &serializeBuffer{}
}

// NewSerializeBufferExpectedSize creates a new buffer for serialization, optimized for an
// expected number of bytes prepended/appended.  This tends to decrease the
// number of memory allocations made by the buffer during writes.
func NewSerializeBufferExpectedSize(expectedPrependLength, expectedAppendLength int) SerializeBuffer {
	return &serializeBuffer{
		data:      make([]byte, expectedPrependLength, expectedPrependLength+expectedAppendLength),
		start:     expectedPrependLength,
		prepended: expectedPrependLength,
		appended:  expectedAppendLength,
	}
}

func (w *serializeBuffer) Bytes() []byte {
	return w.data[w.start:]
}

func (w *serializeBuffer) PrependBytes(num int) ([]byte, error) {
	if num < 0 {
		panic("num < 0")
	}
	if w.start < num {
		toPrepend := w.prepended
		if toPrepend < num {
			toPrepend = num
		}
		w.prepended += toPrepend
		length := cap(w.data) + toPrepend
		newData := make([]byte, length)
		newStart := w.start + toPrepend
		copy(newData[newStart:], w.data[w.start:])
		w.start = newStart
		w.data = newData[:toPrepend+len(w.data)]
	}
	w.start -= num
	return w.data[w.start : w.start+num], nil
}

func (w *serializeBuffer) AppendBytes(num int) ([]byte, error) {
	if num < 0 {
		panic("num < 0")
	}
	initialLength := len(w.data)
	if cap(w.data)-initialLength < num {
		toAppend := w.appended
		if toAppend < num {
			toAppend = num
		}
		w.appended += toAppend
		newData := make([]byte, cap(w.data)+toAppend)
		copy(newData[w.start:], w.data[w.start:])
		w.data = newData[:initialLength]
	}
	// Grow the buffer.  We know it'll be under capacity given above.
	w.data = w.data[:initialLength+num]
	return w.data[initialLength:], nil
}

func (w *serializeBuffer) Clear() error {
	w.start = w.prepended
	w.data = w.data[:w.start]
	w.layers = w.layers[:0]
	return nil
}

func (w *serializeBuffer) Layers() []LayerType {
	return w.layers
}

func (w *serializeBuffer) PushLayer(l LayerType) {
	w.layers = append(w.layers, l)
}

// SerializeLayers clears the given write buffer, then writes all layers into it so
// they correctly wrap each other.  Note that by clearing the buffer, it
// invalidates all slices previously returned by w.Bytes()
//
// Example:
//   buf := gopacket.NewSerializeBuffer()
//   opts := gopacket.SerializeOptions{}
//   gopacket.SerializeLayers(buf, opts, a, b, c)
//   firstPayload := buf.Bytes()  // contains byte representation of a(b(c))
//   gopacket.SerializeLayers(buf, opts, d, e, f)
//   secondPayload := buf.Bytes()  // contains byte representation of d(e(f)). firstPayload is now invalidated, since the SerializeLayers call Clears buf.
func SerializeLayers(w SerializeBuffer, opts SerializeOptions, layers ...SerializableLayer) error {
	w.Clear()
	for i := len(layers) - 1; i >= 0; i-- {
		layer := layers[i]
		err := layer.SerializeTo(w, opts)
		if err != nil {
			return err
		}
		w.PushLayer(layer.LayerType())
	}
	return nil
}

// SerializePacket is a convenience function that calls SerializeLayers
// on packet's Layers().
// It returns an error if one of the packet layers is not a SerializableLayer.
func SerializePacket(buf SerializeBuffer, opts SerializeOptions, packet Packet) error {
	sls := []SerializableLayer{}
	for _, layer := range packet.Layers() {
		sl, ok := layer.(SerializableLayer)
		if !ok {
			return fmt.Errorf("layer %s is not serializable", layer.LayerType().String())
		}
		sls = append(sls, sl)
	}
	return SerializeLayers(buf, opts, sls...)
}
