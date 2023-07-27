//go:build !mips && !mips64 && !ppc64 && !s390x && !amd64 && !386 && !arm && !arm64 && !loong64 && !mipsle && !mips64le && !ppc64le && !riscv64 && !wasm
// +build !mips,!mips64,!ppc64,!s390x,!amd64,!386,!arm,!arm64,!loong64,!mipsle,!mips64le,!ppc64le,!riscv64,!wasm

// This file is a fallback, so that package native doesn't break
// the instant the Go project adds support for a new architecture.
//

package native

import (
	"encoding/binary"
	"log"
	"runtime"
	"unsafe"
)

var Endian binary.ByteOrder

var IsBigEndian bool

func init() {
	b := uint16(0xff) // one byte
	if *(*byte)(unsafe.Pointer(&b)) == 0 {
		Endian = binary.BigEndian
		IsBigEndian = true
	} else {
		Endian = binary.LittleEndian
		IsBigEndian = false
	}
	log.Printf("github.com/josharian/native: unrecognized arch %v (%v), please file an issue", runtime.GOARCH, Endian)
}
