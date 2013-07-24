package utils

import (
	"net/http"
)

type HTTPRequestError struct {
	Message    string
	StatusCode int
}

func (e *HTTPRequestError) Error() string {
	return e.Message
}

func NewHTTPRequestError(msg string, resp *http.Response) error {
	return &HTTPRequestError{Message: msg, StatusCode: resp.StatusCode}
}
