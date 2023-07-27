//go:build amd64 || 386 || arm || arm64 || loong64 || mipsle || mips64le || ppc64le || riscv64 || wasm
// +build amd64 386 arm arm64 loong64 mipsle mips64le ppc64le riscv64 wasm

package native

import "encoding/binary"

// Endian is the encoding/binary.ByteOrder implementation for the
// current CPU's native byte order.
var Endian = binary.LittleEndian

// IsBigEndian is whether the current CPU's native byte order is big
// endian.
const IsBigEndian = false
