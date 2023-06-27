//go:build mips || mips64 || ppc64 || s390x
// +build mips mips64 ppc64 s390x

package native

import "encoding/binary"

// Endian is the encoding/binary.ByteOrder implementation for the
// current CPU's native byte order.
var Endian = binary.BigEndian

// IsBigEndian is whether the current CPU's native byte order is big
// endian.
const IsBigEndian = true
