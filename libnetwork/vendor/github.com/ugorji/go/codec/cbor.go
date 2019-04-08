// Copyright (c) 2012-2018 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

import (
	"math"
	"reflect"
	"time"
)

const (
	cborMajorUint byte = iota
	cborMajorNegInt
	cborMajorBytes
	cborMajorText
	cborMajorArray
	cborMajorMap
	cborMajorTag
	cborMajorOther
)

const (
	cborBdFalse byte = 0xf4 + iota
	cborBdTrue
	cborBdNil
	cborBdUndefined
	cborBdExt
	cborBdFloat16
	cborBdFloat32
	cborBdFloat64
)

const (
	cborBdIndefiniteBytes  byte = 0x5f
	cborBdIndefiniteString      = 0x7f
	cborBdIndefiniteArray       = 0x9f
	cborBdIndefiniteMap         = 0xbf
	cborBdBreak                 = 0xff
)

// These define some in-stream descriptors for
// manual encoding e.g. when doing explicit indefinite-length
const (
	CborStreamBytes  byte = 0x5f
	CborStreamString      = 0x7f
	CborStreamArray       = 0x9f
	CborStreamMap         = 0xbf
	CborStreamBreak       = 0xff
)

const (
	cborBaseUint   byte = 0x00
	cborBaseNegInt      = 0x20
	cborBaseBytes       = 0x40
	cborBaseString      = 0x60
	cborBaseArray       = 0x80
	cborBaseMap         = 0xa0
	cborBaseTag         = 0xc0
	cborBaseSimple      = 0xe0
)

func cbordesc(bd byte) string {
	switch bd {
	case cborBdNil:
		return "nil"
	case cborBdFalse:
		return "false"
	case cborBdTrue:
		return "true"
	case cborBdFloat16, cborBdFloat32, cborBdFloat64:
		return "float"
	case cborBdIndefiniteBytes:
		return "bytes*"
	case cborBdIndefiniteString:
		return "string*"
	case cborBdIndefiniteArray:
		return "array*"
	case cborBdIndefiniteMap:
		return "map*"
	default:
		switch {
		case bd >= cborBaseUint && bd < cborBaseNegInt:
			return "(u)int"
		case bd >= cborBaseNegInt && bd < cborBaseBytes:
			return "int"
		case bd >= cborBaseBytes && bd < cborBaseString:
			return "bytes"
		case bd >= cborBaseString && bd < cborBaseArray:
			return "string"
		case bd >= cborBaseArray && bd < cborBaseMap:
			return "array"
		case bd >= cborBaseMap && bd < cborBaseTag:
			return "map"
		case bd >= cborBaseTag && bd < cborBaseSimple:
			return "ext"
		default:
			return "unknown"
		}
	}
}

// -------------------

type cborEncDriver struct {
	noBuiltInTypes
	encDriverNoopContainerWriter
	// encNoSeparator
	e *Encoder
	w encWriter
	h *CborHandle
	x [8]byte
	_ [3]uint64 // padding
}

func (e *cborEncDriver) EncodeNil() {
	e.w.writen1(cborBdNil)
}

func (e *cborEncDriver) EncodeBool(b bool) {
	if b {
		e.w.writen1(cborBdTrue)
	} else {
		e.w.writen1(cborBdFalse)
	}
}

func (e *cborEncDriver) EncodeFloat32(f float32) {
	e.w.writen1(cborBdFloat32)
	bigenHelper{e.x[:4], e.w}.writeUint32(math.Float32bits(f))
}

func (e *cborEncDriver) EncodeFloat64(f float64) {
	e.w.writen1(cborBdFloat64)
	bigenHelper{e.x[:8], e.w}.writeUint64(math.Float64bits(f))
}

func (e *cborEncDriver) encUint(v uint64, bd byte) {
	if v <= 0x17 {
		e.w.writen1(byte(v) + bd)
	} else if v <= math.MaxUint8 {
		e.w.writen2(bd+0x18, uint8(v))
	} else if v <= math.MaxUint16 {
		e.w.writen1(bd + 0x19)
		bigenHelper{e.x[:2], e.w}.writeUint16(uint16(v))
	} else if v <= math.MaxUint32 {
		e.w.writen1(bd + 0x1a)
		bigenHelper{e.x[:4], e.w}.writeUint32(uint32(v))
	} else { // if v <= math.MaxUint64 {
		e.w.writen1(bd + 0x1b)
		bigenHelper{e.x[:8], e.w}.writeUint64(v)
	}
}

func (e *cborEncDriver) EncodeInt(v int64) {
	if v < 0 {
		e.encUint(uint64(-1-v), cborBaseNegInt)
	} else {
		e.encUint(uint64(v), cborBaseUint)
	}
}

func (e *cborEncDriver) EncodeUint(v uint64) {
	e.encUint(v, cborBaseUint)
}

func (e *cborEncDriver) encLen(bd byte, length int) {
	e.encUint(uint64(length), bd)
}

func (e *cborEncDriver) EncodeTime(t time.Time) {
	if t.IsZero() {
		e.EncodeNil()
	} else if e.h.TimeRFC3339 {
		e.encUint(0, cborBaseTag)
		e.EncodeString(cUTF8, t.Format(time.RFC3339Nano))
	} else {
		e.encUint(1, cborBaseTag)
		t = t.UTC().Round(time.Microsecond)
		sec, nsec := t.Unix(), uint64(t.Nanosecond())
		if nsec == 0 {
			e.EncodeInt(sec)
		} else {
			e.EncodeFloat64(float64(sec) + float64(nsec)/1e9)
		}
	}
}

func (e *cborEncDriver) EncodeExt(rv interface{}, xtag uint64, ext Ext, en *Encoder) {
	e.encUint(uint64(xtag), cborBaseTag)
	if v := ext.ConvertExt(rv); v == nil {
		e.EncodeNil()
	} else {
		en.encode(v)
	}
}

func (e *cborEncDriver) EncodeRawExt(re *RawExt, en *Encoder) {
	e.encUint(uint64(re.Tag), cborBaseTag)
	if false && re.Data != nil {
		en.encode(re.Data)
	} else if re.Value != nil {
		en.encode(re.Value)
	} else {
		e.EncodeNil()
	}
}

func (e *cborEncDriver) WriteArrayStart(length int) {
	if e.h.IndefiniteLength {
		e.w.writen1(cborBdIndefiniteArray)
	} else {
		e.encLen(cborBaseArray, length)
	}
}

func (e *cborEncDriver) WriteMapStart(length int) {
	if e.h.IndefiniteLength {
		e.w.writen1(cborBdIndefiniteMap)
	} else {
		e.encLen(cborBaseMap, length)
	}
}

func (e *cborEncDriver) WriteMapEnd() {
	if e.h.IndefiniteLength {
		e.w.writen1(cborBdBreak)
	}
}

func (e *cborEncDriver) WriteArrayEnd() {
	if e.h.IndefiniteLength {
		e.w.writen1(cborBdBreak)
	}
}

func (e *cborEncDriver) EncodeString(c charEncoding, v string) {
	e.encStringBytesS(cborBaseString, v)
}

func (e *cborEncDriver) EncodeStringBytes(c charEncoding, v []byte) {
	if v == nil {
		e.EncodeNil()
	} else if c == cRAW {
		e.encStringBytesS(cborBaseBytes, stringView(v))
	} else {
		e.encStringBytesS(cborBaseString, stringView(v))
	}
}

func (e *cborEncDriver) encStringBytesS(bb byte, v string) {
	if e.h.IndefiniteLength {
		if bb == cborBaseBytes {
			e.w.writen1(cborBdIndefiniteBytes)
		} else {
			e.w.writen1(cborBdIndefiniteString)
		}
		blen := len(v) / 4
		if blen == 0 {
			blen = 64
		} else if blen > 1024 {
			blen = 1024
		}
		for i := 0; i < len(v); {
			var v2 string
			i2 := i + blen
			if i2 < len(v) {
				v2 = v[i:i2]
			} else {
				v2 = v[i:]
			}
			e.encLen(bb, len(v2))
			e.w.writestr(v2)
			i = i2
		}
		e.w.writen1(cborBdBreak)
	} else {
		e.encLen(bb, len(v))
		e.w.writestr(v)
	}
}

// ----------------------

type cborDecDriver struct {
	d *Decoder
	h *CborHandle
	r decReader
	// b      [scratchByteArrayLen]byte
	br     bool // bytes reader
	bdRead bool
	bd     byte
	noBuiltInTypes
	// decNoSeparator
	decDriverNoopContainerReader
	_ [3]uint64 // padding
}

func (d *cborDecDriver) readNextBd() {
	d.bd = d.r.readn1()
	d.bdRead = true
}

func (d *cborDecDriver) uncacheRead() {
	if d.bdRead {
		d.r.unreadn1()
		d.bdRead = false
	}
}

func (d *cborDecDriver) ContainerType() (vt valueType) {
	if !d.bdRead {
		d.readNextBd()
	}
	if d.bd == cborBdNil {
		return valueTypeNil
	} else if d.bd == cborBdIndefiniteBytes || (d.bd >= cborBaseBytes && d.bd < cborBaseString) {
		return valueTypeBytes
	} else if d.bd == cborBdIndefiniteString || (d.bd >= cborBaseString && d.bd < cborBaseArray) {
		return valueTypeString
	} else if d.bd == cborBdIndefiniteArray || (d.bd >= cborBaseArray && d.bd < cborBaseMap) {
		return valueTypeArray
	} else if d.bd == cborBdIndefiniteMap || (d.bd >= cborBaseMap && d.bd < cborBaseTag) {
		return valueTypeMap
	}
	// else {
	// d.d.errorf("isContainerType: unsupported parameter: %v", vt)
	// }
	return valueTypeUnset
}

func (d *cborDecDriver) TryDecodeAsNil() bool {
	if !d.bdRead {
		d.readNextBd()
	}
	// treat Nil and Undefined as nil values
	if d.bd == cborBdNil || d.bd == cborBdUndefined {
		d.bdRead = false
		return true
	}
	return false
}

func (d *cborDecDriver) CheckBreak() bool {
	if !d.bdRead {
		d.readNextBd()
	}
	if d.bd == cborBdBreak {
		d.bdRead = false
		return true
	}
	return false
}

func (d *cborDecDriver) decUint() (ui uint64) {
	v := d.bd & 0x1f
	if v <= 0x17 {
		ui = uint64(v)
	} else {
		if v == 0x18 {
			ui = uint64(d.r.readn1())
		} else if v == 0x19 {
			ui = uint64(bigen.Uint16(d.r.readx(2)))
		} else if v == 0x1a {
			ui = uint64(bigen.Uint32(d.r.readx(4)))
		} else if v == 0x1b {
			ui = uint64(bigen.Uint64(d.r.readx(8)))
		} else {
			d.d.errorf("invalid descriptor decoding uint: %x/%s", d.bd, cbordesc(d.bd))
			return
		}
	}
	return
}

func (d *cborDecDriver) decCheckInteger() (neg bool) {
	if !d.bdRead {
		d.readNextBd()
	}
	major := d.bd >> 5
	if major == cborMajorUint {
	} else if major == cborMajorNegInt {
		neg = true
	} else {
		d.d.errorf("not an integer - invalid major %v from descriptor %x/%s", major, d.bd, cbordesc(d.bd))
		return
	}
	return
}

func (d *cborDecDriver) DecodeInt64() (i int64) {
	neg := d.decCheckInteger()
	ui := d.decUint()
	// check if this number can be converted to an int without overflow
	if neg {
		i = -(chkOvf.SignedIntV(ui + 1))
	} else {
		i = chkOvf.SignedIntV(ui)
	}
	d.bdRead = false
	return
}

func (d *cborDecDriver) DecodeUint64() (ui uint64) {
	if d.decCheckInteger() {
		d.d.errorf("assigning negative signed value to unsigned type")
		return
	}
	ui = d.decUint()
	d.bdRead = false
	return
}

func (d *cborDecDriver) DecodeFloat64() (f float64) {
	if !d.bdRead {
		d.readNextBd()
	}
	if bd := d.bd; bd == cborBdFloat16 {
		f = float64(math.Float32frombits(halfFloatToFloatBits(bigen.Uint16(d.r.readx(2)))))
	} else if bd == cborBdFloat32 {
		f = float64(math.Float32frombits(bigen.Uint32(d.r.readx(4))))
	} else if bd == cborBdFloat64 {
		f = math.Float64frombits(bigen.Uint64(d.r.readx(8)))
	} else if bd >= cborBaseUint && bd < cborBaseBytes {
		f = float64(d.DecodeInt64())
	} else {
		d.d.errorf("float only valid from float16/32/64 - invalid descriptor %x/%s", bd, cbordesc(bd))
		return
	}
	d.bdRead = false
	return
}

// bool can be decoded from bool only (single byte).
func (d *cborDecDriver) DecodeBool() (b bool) {
	if !d.bdRead {
		d.readNextBd()
	}
	if bd := d.bd; bd == cborBdTrue {
		b = true
	} else if bd == cborBdFalse {
	} else {
		d.d.errorf("not bool - %s %x/%s", msgBadDesc, d.bd, cbordesc(d.bd))
		return
	}
	d.bdRead = false
	return
}

func (d *cborDecDriver) ReadMapStart() (length int) {
	if !d.bdRead {
		d.readNextBd()
	}
	d.bdRead = false
	if d.bd == cborBdIndefiniteMap {
		return -1
	}
	return d.decLen()
}

func (d *cborDecDriver) ReadArrayStart() (length int) {
	if !d.bdRead {
		d.readNextBd()
	}
	d.bdRead = false
	if d.bd == cborBdIndefiniteArray {
		return -1
	}
	return d.decLen()
}

func (d *cborDecDriver) decLen() int {
	return int(d.decUint())
}

func (d *cborDecDriver) decAppendIndefiniteBytes(bs []byte) []byte {
	d.bdRead = false
	for {
		if d.CheckBreak() {
			break
		}
		if major := d.bd >> 5; major != cborMajorBytes && major != cborMajorText {
			d.d.errorf("expect bytes/string major type in indefinite string/bytes;"+
				" got major %v from descriptor %x/%x", major, d.bd, cbordesc(d.bd))
			return nil
		}
		n := d.decLen()
		oldLen := len(bs)
		newLen := oldLen + n
		if newLen > cap(bs) {
			bs2 := make([]byte, newLen, 2*cap(bs)+n)
			copy(bs2, bs)
			bs = bs2
		} else {
			bs = bs[:newLen]
		}
		d.r.readb(bs[oldLen:newLen])
		// bs = append(bs, d.r.readn()...)
		d.bdRead = false
	}
	d.bdRead = false
	return bs
}

func (d *cborDecDriver) DecodeBytes(bs []byte, zerocopy bool) (bsOut []byte) {
	if !d.bdRead {
		d.readNextBd()
	}
	if d.bd == cborBdNil || d.bd == cborBdUndefined {
		d.bdRead = false
		return nil
	}
	if d.bd == cborBdIndefiniteBytes || d.bd == cborBdIndefiniteString {
		d.bdRead = false
		if bs == nil {
			if zerocopy {
				return d.decAppendIndefiniteBytes(d.d.b[:0])
			}
			return d.decAppendIndefiniteBytes(zeroByteSlice)
		}
		return d.decAppendIndefiniteBytes(bs[:0])
	}
	// check if an "array" of uint8's (see ContainerType for how to infer if an array)
	if d.bd == cborBdIndefiniteArray || (d.bd >= cborBaseArray && d.bd < cborBaseMap) {
		bsOut, _ = fastpathTV.DecSliceUint8V(bs, true, d.d)
		return
	}
	clen := d.decLen()
	d.bdRead = false
	if zerocopy {
		if d.br {
			return d.r.readx(clen)
		} else if len(bs) == 0 {
			bs = d.d.b[:]
		}
	}
	return decByteSlice(d.r, clen, d.h.MaxInitLen, bs)
}

func (d *cborDecDriver) DecodeString() (s string) {
	return string(d.DecodeBytes(d.d.b[:], true))
}

func (d *cborDecDriver) DecodeStringAsBytes() (s []byte) {
	return d.DecodeBytes(d.d.b[:], true)
}

func (d *cborDecDriver) DecodeTime() (t time.Time) {
	if !d.bdRead {
		d.readNextBd()
	}
	if d.bd == cborBdNil || d.bd == cborBdUndefined {
		d.bdRead = false
		return
	}
	xtag := d.decUint()
	d.bdRead = false
	return d.decodeTime(xtag)
}

func (d *cborDecDriver) decodeTime(xtag uint64) (t time.Time) {
	if !d.bdRead {
		d.readNextBd()
	}
	switch xtag {
	case 0:
		var err error
		if t, err = time.Parse(time.RFC3339, stringView(d.DecodeStringAsBytes())); err != nil {
			d.d.errorv(err)
		}
	case 1:
		// decode an int64 or a float, and infer time.Time from there.
		// for floats, round to microseconds, as that is what is guaranteed to fit well.
		switch {
		case d.bd == cborBdFloat16, d.bd == cborBdFloat32:
			f1, f2 := math.Modf(d.DecodeFloat64())
			t = time.Unix(int64(f1), int64(f2*1e9))
		case d.bd == cborBdFloat64:
			f1, f2 := math.Modf(d.DecodeFloat64())
			t = time.Unix(int64(f1), int64(f2*1e9))
		case d.bd >= cborBaseUint && d.bd < cborBaseNegInt,
			d.bd >= cborBaseNegInt && d.bd < cborBaseBytes:
			t = time.Unix(d.DecodeInt64(), 0)
		default:
			d.d.errorf("time.Time can only be decoded from a number (or RFC3339 string)")
		}
	default:
		d.d.errorf("invalid tag for time.Time - expecting 0 or 1, got 0x%x", xtag)
	}
	t = t.UTC().Round(time.Microsecond)
	return
}

func (d *cborDecDriver) DecodeExt(rv interface{}, xtag uint64, ext Ext) (realxtag uint64) {
	if !d.bdRead {
		d.readNextBd()
	}
	u := d.decUint()
	d.bdRead = false
	realxtag = u
	if ext == nil {
		re := rv.(*RawExt)
		re.Tag = realxtag
		d.d.decode(&re.Value)
	} else if xtag != realxtag {
		d.d.errorf("Wrong extension tag. Got %b. Expecting: %v", realxtag, xtag)
		return
	} else {
		var v interface{}
		d.d.decode(&v)
		ext.UpdateExt(rv, v)
	}
	d.bdRead = false
	return
}

func (d *cborDecDriver) DecodeNaked() {
	if !d.bdRead {
		d.readNextBd()
	}

	n := d.d.n
	var decodeFurther bool

	switch d.bd {
	case cborBdNil:
		n.v = valueTypeNil
	case cborBdFalse:
		n.v = valueTypeBool
		n.b = false
	case cborBdTrue:
		n.v = valueTypeBool
		n.b = true
	case cborBdFloat16, cborBdFloat32, cborBdFloat64:
		n.v = valueTypeFloat
		n.f = d.DecodeFloat64()
	case cborBdIndefiniteBytes:
		n.v = valueTypeBytes
		n.l = d.DecodeBytes(nil, false)
	case cborBdIndefiniteString:
		n.v = valueTypeString
		n.s = d.DecodeString()
	case cborBdIndefiniteArray:
		n.v = valueTypeArray
		decodeFurther = true
	case cborBdIndefiniteMap:
		n.v = valueTypeMap
		decodeFurther = true
	default:
		switch {
		case d.bd >= cborBaseUint && d.bd < cborBaseNegInt:
			if d.h.SignedInteger {
				n.v = valueTypeInt
				n.i = d.DecodeInt64()
			} else {
				n.v = valueTypeUint
				n.u = d.DecodeUint64()
			}
		case d.bd >= cborBaseNegInt && d.bd < cborBaseBytes:
			n.v = valueTypeInt
			n.i = d.DecodeInt64()
		case d.bd >= cborBaseBytes && d.bd < cborBaseString:
			n.v = valueTypeBytes
			n.l = d.DecodeBytes(nil, false)
		case d.bd >= cborBaseString && d.bd < cborBaseArray:
			n.v = valueTypeString
			n.s = d.DecodeString()
		case d.bd >= cborBaseArray && d.bd < cborBaseMap:
			n.v = valueTypeArray
			decodeFurther = true
		case d.bd >= cborBaseMap && d.bd < cborBaseTag:
			n.v = valueTypeMap
			decodeFurther = true
		case d.bd >= cborBaseTag && d.bd < cborBaseSimple:
			n.v = valueTypeExt
			n.u = d.decUint()
			n.l = nil
			if n.u == 0 || n.u == 1 {
				d.bdRead = false
				n.v = valueTypeTime
				n.t = d.decodeTime(n.u)
			}
			// d.bdRead = false
			// d.d.decode(&re.Value) // handled by decode itself.
			// decodeFurther = true
		default:
			d.d.errorf("decodeNaked: Unrecognized d.bd: 0x%x", d.bd)
			return
		}
	}

	if !decodeFurther {
		d.bdRead = false
	}
	return
}

// -------------------------

// CborHandle is a Handle for the CBOR encoding format,
// defined at http://tools.ietf.org/html/rfc7049 and documented further at http://cbor.io .
//
// CBOR is comprehensively supported, including support for:
//   - indefinite-length arrays/maps/bytes/strings
//   - (extension) tags in range 0..0xffff (0 .. 65535)
//   - half, single and double-precision floats
//   - all numbers (1, 2, 4 and 8-byte signed and unsigned integers)
//   - nil, true, false, ...
//   - arrays and maps, bytes and text strings
//
// None of the optional extensions (with tags) defined in the spec are supported out-of-the-box.
// Users can implement them as needed (using SetExt), including spec-documented ones:
//   - timestamp, BigNum, BigFloat, Decimals,
//   - Encoded Text (e.g. URL, regexp, base64, MIME Message), etc.
type CborHandle struct {
	binaryEncodingType
	noElemSeparators
	BasicHandle

	// IndefiniteLength=true, means that we encode using indefinitelength
	IndefiniteLength bool

	// TimeRFC3339 says to encode time.Time using RFC3339 format.
	// If unset, we encode time.Time using seconds past epoch.
	TimeRFC3339 bool

	// _ [1]uint64 // padding
}

// Name returns the name of the handle: cbor
func (h *CborHandle) Name() string { return "cbor" }

// SetInterfaceExt sets an extension
func (h *CborHandle) SetInterfaceExt(rt reflect.Type, tag uint64, ext InterfaceExt) (err error) {
	return h.SetExt(rt, tag, &extWrapper{bytesExtFailer{}, ext})
}

func (h *CborHandle) newEncDriver(e *Encoder) encDriver {
	return &cborEncDriver{e: e, w: e.w, h: h}
}

func (h *CborHandle) newDecDriver(d *Decoder) decDriver {
	return &cborDecDriver{d: d, h: h, r: d.r, br: d.bytes}
}

func (e *cborEncDriver) reset() {
	e.w = e.e.w
}

func (d *cborDecDriver) reset() {
	d.r, d.br = d.d.r, d.d.bytes
	d.bd, d.bdRead = 0, false
}

var _ decDriver = (*cborDecDriver)(nil)
var _ encDriver = (*cborEncDriver)(nil)
