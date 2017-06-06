package pad4

import (
	"bytes"
	"io"
)

// A Writer is an io.WriteCloser. Writes are padded with zeros to 4 byte boundary
type Writer struct {
	w     io.Writer
	count int
}

// Write writes output
func (pad Writer) Write(p []byte) (int, error) {
	n, err := pad.w.Write(p)
	if err != nil {
		return 0, err
	}
	pad.count += n
	return n, nil
}

// Close adds the padding
func (pad Writer) Close() error {
	mod4 := pad.count & 3
	if mod4 == 0 {
		return nil
	}
	zero := make([]byte, 4-mod4)
	buf := bytes.NewBuffer(zero)
	n, err := io.Copy(pad.w, buf)
	if err != nil {
		return err
	}
	pad.count += int(n)
	return nil
}

// NewWriter provides a new io.WriteCloser that zero pads the
// output to a multiple of four bytes
func NewWriter(w io.Writer) *Writer {
	pad := new(Writer)
	pad.w = w
	return pad
}
