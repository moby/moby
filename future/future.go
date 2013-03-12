package future

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os/exec"
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

func HumanDuration(d time.Duration) string {
	if seconds := int(d.Seconds()); seconds < 1 {
		return "Less than a second"
	} else if seconds < 60 {
		return fmt.Sprintf("%d seconds", seconds)
	} else if minutes := int(d.Minutes()); minutes == 1 {
		return "About a minute"
	} else if minutes < 60 {
		return fmt.Sprintf("%d minutes", minutes)
	} else if hours := int(d.Hours()); hours == 1 {
		return "About an hour"
	} else if hours < 48 {
		return fmt.Sprintf("%d hours", hours)
	} else if hours < 24*7*2 {
		return fmt.Sprintf("%d days", hours/24)
	} else if hours < 24*30*3 {
		return fmt.Sprintf("%d weeks", hours/24/7)
	} else if hours < 24*365*2 {
		return fmt.Sprintf("%d months", hours/24/30)
	}
	return fmt.Sprintf("%d years", d.Hours()/24/365)
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

// Curl makes an http request by executing the unix command 'curl', and returns
// the body of the response. If `stderr` is not nil, a progress bar will be
// written to it.
func Curl(url string, stderr io.Writer) (io.Reader, error) {
	curl := exec.Command("curl", "-#", "-L", "--fail", url)
	output, err := curl.StdoutPipe()
	if err != nil {
		return nil, err
	}
	curl.Stderr = stderr
	if err := curl.Start(); err != nil {
		return nil, err
	}
	if err := curl.Wait(); err != nil {
		return nil, err
	}
	return output, nil
}

// Request a given URL and return an io.Reader
func Download(url string, stderr io.Writer) (*http.Response, error) {
	var resp *http.Response
	var err error = nil

	fmt.Fprintf(stderr, "Download start\n") // FIXME: Replace with progress bar
	if resp, err = http.Get(url); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, errors.New("Got HTTP status code >= 400: " + resp.Status)
	}
	fmt.Fprintf(stderr, "Download end\n") // FIXME: Replace with progress bar
	return resp, nil
}

// Reader with progress bar
type progressReader struct {
	reader io.ReadCloser   // Stream to read from
	output io.Writer   // Where to send progress bar to
	read_total   int    // Expected stream length (bytes)
	read_progress int  // How much has been read so far (bytes)
}
func (r *progressReader) Read(p []byte) (n int, err error) {
	read, err := io.ReadCloser(r.reader).Read(p)
	// FIXME: Don't print progress bar on every read
	r.read_progress += read
	fmt.Fprintf(r.output, "%d/%d (%.2f%%)\n", 
		r.read_progress,
		r.read_total,
		float64(r.read_progress) / float64(r.read_total) * 100)
	return read, err
}
func (r *progressReader) Close() error {
	return io.ReadCloser(r.reader).Close()
}
func ProgressReader(r io.ReadCloser, size int, output io.Writer) *progressReader {
	return &progressReader{r, output, size, 0}
}
