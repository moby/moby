package main

import (
	"fmt"
	"io"
	"os"
)

func fatal(err error) {
	fmt.Fprint(os.Stderr, err)
	os.Exit(1)
}

// flush consumes all the bytes from the reader discarding
// any errors
func flush(r io.Reader) (bytes int64, err error) {
	return io.Copy(io.Discard, r)
}
