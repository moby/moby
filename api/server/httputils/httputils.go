package httputils // import "github.com/docker/docker/api/server/httputils"

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
)

// APIVersionKey is the client's requested API version.
type APIVersionKey struct{}

// APIFunc is an adapter to allow the use of ordinary functions as Docker API endpoints.
// Any function that has the appropriate signature can be registered as an API endpoint (e.g. getVersion).
type APIFunc func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error

// HijackConnection interrupts the http response writer to get the
// underlying connection and operate with it.
func HijackConnection(w http.ResponseWriter) (io.ReadCloser, io.Writer, error) {
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return nil, nil, err
	}
	// Flush the options to make sure the client sets the raw mode
	_, _ = conn.Write([]byte{})
	return conn, conn, nil
}

// CloseStreams ensures that a list for http streams are properly closed.
func CloseStreams(streams ...interface{}) {
	for _, stream := range streams {
		if tcpc, ok := stream.(interface {
			CloseWrite() error
		}); ok {
			_ = tcpc.CloseWrite()
		} else if closer, ok := stream.(io.Closer); ok {
			_ = closer.Close()
		}
	}
}

// CheckForJSON makes sure that the request's Content-Type is application/json.
func CheckForJSON(r *http.Request) error {
	ct := r.Header.Get("Content-Type")

	// No Content-Type header is ok as long as there's no Body
	if ct == "" && (r.Body == nil || r.ContentLength == 0) {
		return nil
	}

	// Otherwise it better be json
	return matchesContentType(ct, "application/json")
}

// ReadJSON validates the request to have the correct content-type, and decodes
// the request's Body into out.
func ReadJSON(r *http.Request, out interface{}) error {
	err := CheckForJSON(r)
	if err != nil {
		return err
	}
	if r.Body == nil || r.ContentLength == 0 {
		// an empty body is not invalid, so don't return an error; see
		// https://lists.w3.org/Archives/Public/ietf-http-wg/2010JulSep/0272.html
		return nil
	}

	dec := json.NewDecoder(r.Body)
	err = dec.Decode(out)
	defer r.Body.Close()
	if err != nil {
		if err == io.EOF {
			return errdefs.InvalidParameter(errors.New("invalid JSON: got EOF while reading request body"))
		}
		return errdefs.InvalidParameter(errors.Wrap(err, "invalid JSON"))
	}

	if dec.More() {
		return errdefs.InvalidParameter(errors.New("unexpected content after JSON"))
	}
	return nil
}

// WriteJSON writes the value v to the http response stream as json with standard json encoding.
func WriteJSON(w http.ResponseWriter, code int, v interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// ParseForm ensures the request form is parsed even with invalid content types.
// If we don't do this, POST method without Content-type (even with empty body) will fail.
func ParseForm(r *http.Request) error {
	if r == nil {
		return nil
	}
	if err := r.ParseForm(); err != nil && !strings.HasPrefix(err.Error(), "mime:") {
		return errdefs.InvalidParameter(err)
	}
	return nil
}

// VersionFromContext returns an API version from the context using APIVersionKey.
// It panics if the context value does not have version.Version type.
func VersionFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if val := ctx.Value(APIVersionKey{}); val != nil {
		return val.(string)
	}

	return ""
}

// matchesContentType validates the content type against the expected one
func matchesContentType(contentType, expectedType string) error {
	mimetype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return errdefs.InvalidParameter(errors.Wrapf(err, "malformed Content-Type header (%s)", contentType))
	}
	if mimetype != expectedType {
		return errdefs.InvalidParameter(errors.Errorf("unsupported Content-Type header (%s): must be '%s'", contentType, expectedType))
	}
	return nil
}
