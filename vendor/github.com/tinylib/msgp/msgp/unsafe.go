//go:build (!purego && !appengine) || (!appengine && purego && unsafe)
// +build !purego,!appengine !appengine,purego,unsafe

package msgp

import (
	"unsafe"
)

// NOTE:
// all of the definition in this file
// should be repeated in appengine.go,
// but without using unsafe

const (
	// spec says int and uint are always
	// the same size, but that int/uint
	// size may not be machine word size
	smallint = unsafe.Sizeof(int(0)) == 4
)

// UnsafeString returns the byte slice as a volatile string
// THIS SHOULD ONLY BE USED BY THE CODE GENERATOR.
// THIS IS EVIL CODE.
// YOU HAVE BEEN WARNED.
func UnsafeString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// UnsafeBytes returns the string as a byte slice
//
// Deprecated:
// Since this code is no longer used by the code generator,
// UnsafeBytes(s) is precisely equivalent to []byte(s)
func UnsafeBytes(s string) []byte {
	return []byte(s)
}
