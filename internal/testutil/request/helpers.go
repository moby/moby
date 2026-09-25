package request

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
)

// ReadBody read the specified ReadCloser content and returns it.
func ReadBody(b io.ReadCloser) ([]byte, error) {
	defer func() { _ = b.Close() }()
	return io.ReadAll(b)
}

// ReadJSONResponse reads a JSON response body into the given variable. it
// returns an error for non-jSON responses, or when failing to unmarshal.
func ReadJSONResponse[T any](resp *http.Response, v *T) error {
	if resp == nil {
		return errors.New("nil *http.Response")
	}
	defer func() { _ = resp.Body.Close() }()

	mt, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mt != "application/json" {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return fmt.Errorf("unexpected Content-Type: '%s' (body: %s)", mt, string(raw))
	}

	return json.NewDecoder(resp.Body).Decode(v)
}
