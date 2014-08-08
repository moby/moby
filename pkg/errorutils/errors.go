package errorutils

import (
	"net/http"
	"fmt"
)

type JSONError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (e *JSONError) Error() string {
	return e.Message
}

func NewHTTPRequestError(msg string, res *http.Response) error {
	return &JSONError{
		Message: msg,
		Code:    res.StatusCode,
	}
}

// An StatusError reports an unsuccessful exit by a command.
type StatusError struct {
	Status     string
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("Status: %s, Code: %d", e.Status, e.StatusCode)
}
