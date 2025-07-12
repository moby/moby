package plugins

import (
	"errors"
	"fmt"
	"net/http"
)

type statusError struct {
	status int
	method string
	err    string
}

// Error returns a formatted string for this error type
func (e *statusError) Error() string {
	return fmt.Sprintf("%s: %v", e.method, e.err)
}

// IsNotFound indicates if the passed in error is from an http.StatusNotFound from the plugin
func IsNotFound(err error) bool {
	return isStatusError(err, http.StatusNotFound)
}

func isStatusError(err error, status int) bool {
	if err == nil {
		return false
	}
	var e *statusError
	if !errors.As(err, &e) {
		return false
	}
	return e.status == status
}
