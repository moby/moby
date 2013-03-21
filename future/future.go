package future

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
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

// Request a given URL and return an io.Reader
func Download(url string, stderr io.Writer) (*http.Response, error) {
	var resp *http.Response
	var err error = nil
	if resp, err = http.Get(url); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, errors.New("Got HTTP status code >= 400: " + resp.Status)
	}
	return resp, nil
}

// Reader with progress bar
type progressReader struct {
	reader        io.ReadCloser // Stream to read from
	output        io.Writer     // Where to send progress bar to
	read_total    int           // Expected stream length (bytes)
	read_progress int           // How much has been read so far (bytes)
	last_update   int           // How many bytes read at least update
}

func (r *progressReader) Read(p []byte) (n int, err error) {
	read, err := io.ReadCloser(r.reader).Read(p)
	r.read_progress += read

	// Only update progress for every 1% read
	update_every := int(0.01 * float64(r.read_total))
	if r.read_progress-r.last_update > update_every || r.read_progress == r.read_total {
		fmt.Fprintf(r.output, "%d/%d (%.0f%%)\r",
			r.read_progress,
			r.read_total,
			float64(r.read_progress)/float64(r.read_total)*100)
		r.last_update = r.read_progress
	}
	// Send newline when complete
	if err == io.EOF {
		fmt.Fprintf(r.output, "\n")
	}

	return read, err
}
func (r *progressReader) Close() error {
	return io.ReadCloser(r.reader).Close()
}
func ProgressReader(r io.ReadCloser, size int, output io.Writer) *progressReader {
	return &progressReader{r, output, size, 0, 0}
}
