package httputils

import (
	"fmt"
	"net/http"

	"github.com/docker/docker/pkg/jsonmessage"
)

// Download requests a given URL and returns an io.Reader
func Download(url string) (resp *http.Response, err error) {
	if resp, err = http.Get(url); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Got HTTP status code >= 400: %s", resp.Status)
	}
	return resp, nil
}

// NewHTTPRequestError returns a JSON response error
func NewHTTPRequestError(msg string, res *http.Response) error {
	return &jsonmessage.JSONError{
		Message: msg,
		Code:    res.StatusCode,
	}
}
