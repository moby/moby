package future

import (
	"fmt"
	"io"
)

func Go(f func() error) chan error {
	ch := make(chan error)
	go func() {
		ch <- f()
	}()
	return ch
}

// Pv wraps an io.Reader such that it is passed through unchanged,
// but logs the number of bytes copied (comparable to the unix command pv)
func Pv(src io.Reader, info io.Writer) io.Reader {
	var totalBytes int
	data := make([]byte, 2048)
	r, w := io.Pipe()
	go func() {
		for {
			if n, err := src.Read(data); err != nil {
				w.CloseWithError(err)
				return
			} else {
				totalBytes += n
				fmt.Fprintf(info, "--> %d bytes\n", totalBytes)
				if _, err = w.Write(data[:n]); err != nil {
					return
				}
			}
		}
	}()
	return r
}
