//go:build tinygo
// +build tinygo

package msgp

// for tinygo, getBytespec just calls calcBytespec
// a simple/slow function with a switch statement -
// doesn't require any heap alloc, moves the space
// requirements into code instad of ram

func getBytespec(v byte) bytespec {
	return calcBytespec(v)
}
