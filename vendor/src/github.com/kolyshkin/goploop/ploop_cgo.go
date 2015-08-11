package ploop

// A few auxiliary helpers to simplify life with CGo

// #include <stdlib.h>
import "C"
import "unsafe"

// cfree frees a C string
func cfree(c *C.char) {
	C.free(unsafe.Pointer(c))
}

// boolToC converts Go bool to C.int
func boolToC(v bool) C.int {
	if v {
		return 1
	}
	return 0
}

// convertSize converts a size in kilobytes to whatever ploop lib is using
func convertSize(size uint64) C.ulonglong {
	return C.ulonglong(size * 2) // kB to 512-byte sectors
}
