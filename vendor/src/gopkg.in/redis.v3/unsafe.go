// +build !appengine

package redis

import (
	"reflect"
	"unsafe"
)

func bytesToString(b []byte) string {
	bytesHeader := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	strHeader := reflect.StringHeader{bytesHeader.Data, bytesHeader.Len}
	return *(*string)(unsafe.Pointer(&strHeader))
}
