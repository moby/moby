package httputils

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/docker/docker/pkg/jsonmessage"
)

var (
	headerRegexp     = regexp.MustCompile(`^(?:(.+)/(.+?))\((.+)\).*$`)
	errInvalidHeader = errors.New("Bad header, should be in format `docker/version (platform)`")
)

// Download requests a given URL and returns an io.Reader.
func Download(url string) (resp *http.Response, err error) {
	if resp, err = http.Get(url); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Got HTTP status code >= 400: %s", resp.Status)
	}
	return resp, nil
}

// NewHTTPRequestError returns a JSON response error.
func NewHTTPRequestError(msg string, res *http.Response) error {
	return &jsonmessage.JSONError{
		Message: msg,
		Code:    res.StatusCode,
	}
}

// ServerHeader contains the server information.
type ServerHeader struct {
	App string // docker
	Ver string // 1.8.0-dev
	OS  string // windows or linux
}

// ParseServerHeader extracts pieces from an HTTP server header
// which is in the format "docker/version (os)" eg docker/1.8.0-dev (windows).
func ParseServerHeader(hdr string) (*ServerHeader, error) {
	matches := headerRegexp.FindStringSubmatch(hdr)
	if len(matches) != 4 {
		return nil, errInvalidHeader
	}
	return &ServerHeader{
		App: strings.TrimSpace(matches[1]),
		Ver: strings.TrimSpace(matches[2]),
		OS:  strings.TrimSpace(matches[3]),
	}, nil
}
