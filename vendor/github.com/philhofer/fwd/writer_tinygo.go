//go:build tinygo
// +build tinygo

package fwd

import (
	"unsafe"
)

// unsafe cast string as []byte
func unsafestr(b string) []byte {
	return unsafe.Slice(unsafe.StringData(b), len(b))
}
