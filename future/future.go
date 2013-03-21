package future

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"math/rand"
	"time"
)

func Seed() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func ComputeId(content io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, content); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)[:8]), nil
}

func randomBytes() io.Reader {
	return bytes.NewBuffer([]byte(fmt.Sprintf("%x", rand.Int())))
}

func RandomId() string {
	id, _ := ComputeId(randomBytes()) // can't fail
	return id
}

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
