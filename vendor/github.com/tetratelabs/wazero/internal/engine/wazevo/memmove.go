package wazevo

import (
	"reflect"
	"unsafe"
)

//go:linkname memmove runtime.memmove
func memmove(_, _ unsafe.Pointer, _ uintptr)

var memmovPtr = reflect.ValueOf(memmove).Pointer()
