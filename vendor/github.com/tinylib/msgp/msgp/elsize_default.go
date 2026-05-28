//go:build !tinygo
// +build !tinygo

package msgp

// size of every object on the wire,
// plus type information. gives us
// constant-time type information
// for traversing composite objects.
var sizes [256]bytespec

func init() {
	for i := 0; i < 256; i++ {
		sizes[i] = calcBytespec(byte(i))
	}
}

// getBytespec gets inlined to a simple array index
func getBytespec(v byte) bytespec {
	return sizes[v]
}
