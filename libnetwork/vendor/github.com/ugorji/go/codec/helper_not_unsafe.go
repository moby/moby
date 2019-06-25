// +build !go1.7 safe appengine

// Copyright (c) 2012-2018 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

import (
	"reflect"
	"sync/atomic"
	"time"
)

const safeMode = true

// stringView returns a view of the []byte as a string.
// In unsafe mode, it doesn't incur allocation and copying caused by conversion.
// In regular safe mode, it is an allocation and copy.
//
// Usage: Always maintain a reference to v while result of this call is in use,
//        and call keepAlive4BytesView(v) at point where done with view.
func stringView(v []byte) string {
	return string(v)
}

// bytesView returns a view of the string as a []byte.
// In unsafe mode, it doesn't incur allocation and copying caused by conversion.
// In regular safe mode, it is an allocation and copy.
//
// Usage: Always maintain a reference to v while result of this call is in use,
//        and call keepAlive4BytesView(v) at point where done with view.
func bytesView(v string) []byte {
	return []byte(v)
}

func definitelyNil(v interface{}) bool {
	// this is a best-effort option.
	// We just return false, so we don't unnecessarily incur the cost of reflection this early.
	return false
}

func rv2i(rv reflect.Value) interface{} {
	return rv.Interface()
}

func rt2id(rt reflect.Type) uintptr {
	return reflect.ValueOf(rt).Pointer()
}

func rv2rtid(rv reflect.Value) uintptr {
	return reflect.ValueOf(rv.Type()).Pointer()
}

func i2rtid(i interface{}) uintptr {
	return reflect.ValueOf(reflect.TypeOf(i)).Pointer()
}

// --------------------------

func isEmptyValue(v reflect.Value, tinfos *TypeInfos, deref, checkStruct bool) bool {
	switch v.Kind() {
	case reflect.Invalid:
		return true
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		if deref {
			if v.IsNil() {
				return true
			}
			return isEmptyValue(v.Elem(), tinfos, deref, checkStruct)
		}
		return v.IsNil()
	case reflect.Struct:
		return isEmptyStruct(v, tinfos, deref, checkStruct)
	}
	return false
}

// --------------------------
// type ptrToRvMap struct{}

// func (*ptrToRvMap) init() {}
// func (*ptrToRvMap) get(i interface{}) reflect.Value {
// 	return reflect.ValueOf(i).Elem()
// }

// --------------------------
type atomicTypeInfoSlice struct { // expected to be 2 words
	v atomic.Value
}

func (x *atomicTypeInfoSlice) load() []rtid2ti {
	i := x.v.Load()
	if i == nil {
		return nil
	}
	return i.([]rtid2ti)
}

func (x *atomicTypeInfoSlice) store(p []rtid2ti) {
	x.v.Store(p)
}

// --------------------------
func (d *Decoder) raw(f *codecFnInfo, rv reflect.Value) {
	rv.SetBytes(d.rawBytes())
}

func (d *Decoder) kString(f *codecFnInfo, rv reflect.Value) {
	rv.SetString(d.d.DecodeString())
}

func (d *Decoder) kBool(f *codecFnInfo, rv reflect.Value) {
	rv.SetBool(d.d.DecodeBool())
}

func (d *Decoder) kTime(f *codecFnInfo, rv reflect.Value) {
	rv.Set(reflect.ValueOf(d.d.DecodeTime()))
}

func (d *Decoder) kFloat32(f *codecFnInfo, rv reflect.Value) {
	fv := d.d.DecodeFloat64()
	if chkOvf.Float32(fv) {
		d.errorf("float32 overflow: %v", fv)
	}
	rv.SetFloat(fv)
}

func (d *Decoder) kFloat64(f *codecFnInfo, rv reflect.Value) {
	rv.SetFloat(d.d.DecodeFloat64())
}

func (d *Decoder) kInt(f *codecFnInfo, rv reflect.Value) {
	rv.SetInt(chkOvf.IntV(d.d.DecodeInt64(), intBitsize))
}

func (d *Decoder) kInt8(f *codecFnInfo, rv reflect.Value) {
	rv.SetInt(chkOvf.IntV(d.d.DecodeInt64(), 8))
}

func (d *Decoder) kInt16(f *codecFnInfo, rv reflect.Value) {
	rv.SetInt(chkOvf.IntV(d.d.DecodeInt64(), 16))
}

func (d *Decoder) kInt32(f *codecFnInfo, rv reflect.Value) {
	rv.SetInt(chkOvf.IntV(d.d.DecodeInt64(), 32))
}

func (d *Decoder) kInt64(f *codecFnInfo, rv reflect.Value) {
	rv.SetInt(d.d.DecodeInt64())
}

func (d *Decoder) kUint(f *codecFnInfo, rv reflect.Value) {
	rv.SetUint(chkOvf.UintV(d.d.DecodeUint64(), uintBitsize))
}

func (d *Decoder) kUintptr(f *codecFnInfo, rv reflect.Value) {
	rv.SetUint(chkOvf.UintV(d.d.DecodeUint64(), uintBitsize))
}

func (d *Decoder) kUint8(f *codecFnInfo, rv reflect.Value) {
	rv.SetUint(chkOvf.UintV(d.d.DecodeUint64(), 8))
}

func (d *Decoder) kUint16(f *codecFnInfo, rv reflect.Value) {
	rv.SetUint(chkOvf.UintV(d.d.DecodeUint64(), 16))
}

func (d *Decoder) kUint32(f *codecFnInfo, rv reflect.Value) {
	rv.SetUint(chkOvf.UintV(d.d.DecodeUint64(), 32))
}

func (d *Decoder) kUint64(f *codecFnInfo, rv reflect.Value) {
	rv.SetUint(d.d.DecodeUint64())
}

// ----------------

func (e *Encoder) kBool(f *codecFnInfo, rv reflect.Value) {
	e.e.EncodeBool(rv.Bool())
}

func (e *Encoder) kTime(f *codecFnInfo, rv reflect.Value) {
	e.e.EncodeTime(rv2i(rv).(time.Time))
}

func (e *Encoder) kString(f *codecFnInfo, rv reflect.Value) {
	e.e.EncodeString(cUTF8, rv.String())
}

func (e *Encoder) kFloat64(f *codecFnInfo, rv reflect.Value) {
	e.e.EncodeFloat64(rv.Float())
}

func (e *Encoder) kFloat32(f *codecFnInfo, rv reflect.Value) {
	e.e.EncodeFloat32(float32(rv.Float()))
}

func (e *Encoder) kInt(f *codecFnInfo, rv reflect.Value) {
	e.e.EncodeInt(rv.Int())
}

func (e *Encoder) kInt8(f *codecFnInfo, rv reflect.Value) {
	e.e.EncodeInt(rv.Int())
}

func (e *Encoder) kInt16(f *codecFnInfo, rv reflect.Value) {
	e.e.EncodeInt(rv.Int())
}

func (e *Encoder) kInt32(f *codecFnInfo, rv reflect.Value) {
	e.e.EncodeInt(rv.Int())
}

func (e *Encoder) kInt64(f *codecFnInfo, rv reflect.Value) {
	e.e.EncodeInt(rv.Int())
}

func (e *Encoder) kUint(f *codecFnInfo, rv reflect.Value) {
	e.e.EncodeUint(rv.Uint())
}

func (e *Encoder) kUint8(f *codecFnInfo, rv reflect.Value) {
	e.e.EncodeUint(rv.Uint())
}

func (e *Encoder) kUint16(f *codecFnInfo, rv reflect.Value) {
	e.e.EncodeUint(rv.Uint())
}

func (e *Encoder) kUint32(f *codecFnInfo, rv reflect.Value) {
	e.e.EncodeUint(rv.Uint())
}

func (e *Encoder) kUint64(f *codecFnInfo, rv reflect.Value) {
	e.e.EncodeUint(rv.Uint())
}

func (e *Encoder) kUintptr(f *codecFnInfo, rv reflect.Value) {
	e.e.EncodeUint(rv.Uint())
}

// // keepAlive4BytesView maintains a reference to the input parameter for bytesView.
// //
// // Usage: call this at point where done with the bytes view.
// func keepAlive4BytesView(v string) {}

// // keepAlive4BytesView maintains a reference to the input parameter for stringView.
// //
// // Usage: call this at point where done with the string view.
// func keepAlive4StringView(v []byte) {}

// func definitelyNil(v interface{}) bool {
// 	rv := reflect.ValueOf(v)
// 	switch rv.Kind() {
// 	case reflect.Invalid:
// 		return true
// 	case reflect.Ptr, reflect.Interface, reflect.Chan, reflect.Slice, reflect.Map, reflect.Func:
// 		return rv.IsNil()
// 	default:
// 		return false
// 	}
// }
