package utils // import "github.com/docker/docker/distribution/utils"

import (
	"io"
)

// CopyBufferDirect is equivalent to `io.CopyBuffer`, but without
// the optimizations to use WriteTo or ReadFrom.
func CopyBufferDirect(dst io.Writer, src io.Reader, buf []byte) (written int64, err error) {
	if buf == nil || len(buf) == 0 {
		panic("empty buffer in CopyBufferDirect")
	}
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = io.ErrShortWrite
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}
