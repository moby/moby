//go:build 386 || amd64p32 || arm || mipsle || mips64p32le

package sys

import (
	"structs"
	"unsafe"
)

// Pointer wraps an unsafe.Pointer to be 64bit to
// conform to the syscall specification.
type Pointer struct {
	structs.HostLayout
	ptr unsafe.Pointer
	pad uint32
}
