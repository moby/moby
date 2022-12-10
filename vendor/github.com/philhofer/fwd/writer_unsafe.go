//go:build !appengine && !tinygo
// +build !appengine,!tinygo

package fwd

import (
	"reflect"
	"unsafe"
)

// unsafe cast string as []byte
func unsafestr(s string) []byte {
	var b []byte
	sHdr := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bHdr := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	bHdr.Data = sHdr.Data
	bHdr.Len = sHdr.Len
	bHdr.Cap = sHdr.Len
	return b
}
