package httputil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

// ErrorJSON is returned with "application/json" content type and non-2XX status code
type ErrorJSON struct {
	Message string `json:"message"`
}

func readAtMost(r io.Reader, maxBytes int) ([]byte, error) {
	lr := &io.LimitedReader{
		R: r,
		N: int64(maxBytes),
	}
	b, err := io.ReadAll(lr)
	if err != nil {
		return b, err
	}
	if lr.N == 0 {
		return b, fmt.Errorf("expected at most %d bytes, got more", maxBytes)
	}
	return b, nil
}

// HTTPStatusErrorBodyMaxLength specifies the maximum length of HTTPStatusError.Body
const HTTPStatusErrorBodyMaxLength = 64 * 1024

// HTTPStatusError is created from non-2XX HTTP response
type HTTPStatusError struct {
	// StatusCode is non-2XX status code
	StatusCode int
	// Body is at most HTTPStatusErrorBodyMaxLength
	Body string
}

// Error implements error.
// If e.Body is a marshalled string of api.ErrorJSON, Error returns ErrorJSON.Message .
// Otherwise Error returns a human-readable string that contains e.StatusCode and e.Body.
func (e *HTTPStatusError) Error() string {
	if e.Body != "" && len(e.Body) < HTTPStatusErrorBodyMaxLength {
		var ej ErrorJSON
		if json.Unmarshal([]byte(e.Body), &ej) == nil {
			return ej.Message
		}
	}
	return fmt.Sprintf("unexpected HTTP status %s, body=%q", http.StatusText(e.StatusCode), e.Body)
}

// Successful returns an error if the status code is not 2xx.
func Successful(resp *http.Response) error {
	if resp == nil {
		return errors.New("nil response")
	}
	if resp.StatusCode/100 != 2 {
		b, _ := readAtMost(resp.Body, HTTPStatusErrorBodyMaxLength)
		return &HTTPStatusError{
			StatusCode: resp.StatusCode,
			Body:       string(b),
		}
	}
	return nil
}

// NewHTTPClient returns an HTTP client configured to communicate with the
// RootlessKit API over the Unix socket at socketPath.
func NewHTTPClient(socketPath string) (*http.Client, error) {
	if _, err := os.Stat(socketPath); err != nil {
		return nil, err
	}
	return &http.Client{
		// Use the dial and idle connection timeouts from http.DefaultTransport.
		//
		// Other defaults are either TCP/TLS-specific or unnecessary for the
		// local Unix socket connection.
		// See https://github.com/golang/go/blob/go1.27.2/src/net/http/transport.go#L42-L58
		Transport: &http.Transport{
			DialContext: unixDialContext(socketPath, &net.Dialer{
				Timeout: 30 * time.Second,
			}),
			IdleConnTimeout: 90 * time.Second,
		},
	}, nil
}

// unixDialContext returns a dial function that connects to socketPath using dialer.
func unixDialContext(socketPath string, dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, "unix", socketPath)
	}
}

// WriteError writes an error.
// WriteError sould not be used if an error may contain sensitive information and the client is not reliable.
func WriteError(w http.ResponseWriter, r *http.Request, err error, ec int) {
	w.WriteHeader(ec)
	w.Header().Set("Content-Type", "application/json")
	e := ErrorJSON{
		Message: err.Error(),
	}
	_ = json.NewEncoder(w).Encode(e)
}
