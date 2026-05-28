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

func NewHTTPClient(socketPath string) (*http.Client, error) {
	if _, err := os.Stat(socketPath); err != nil {
		return nil, err
	}
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		},
	}, nil
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
