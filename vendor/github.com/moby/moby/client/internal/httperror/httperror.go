// Package httperror classifies errors using HTTP status codes.
package httperror

import (
	"errors"
	"net/http"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/errdefs/pkg/errhttp"
)

type httpError struct {
	err    error
	errDef error
}

func (e *httpError) Error() string {
	return e.err.Error()
}

func (e *httpError) Unwrap() error {
	return e.err
}

func (e *httpError) Is(target error) bool {
	return errors.Is(e.errDef, target)
}

// FromStatusCode classifies err using the HTTP status code while preserving
// its message and error chain.
func FromStatusCode(err error, statusCode int) error {
	if err == nil || statusCode == 0 {
		return err
	}
	if statusCode >= http.StatusOK && statusCode < http.StatusBadRequest {
		// The HTTP status indicates success or redirection, so err may originate
		// from client-side processing. Preserve it without adding a classification.
		return err
	}
	errDef := errhttp.ToNative(statusCode)
	if cerrdefs.IsUnknown(errDef) {
		switch {
		case statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError:
			errDef = cerrdefs.ErrInvalidArgument
		case statusCode >= http.StatusInternalServerError && statusCode < 600:
			errDef = cerrdefs.ErrInternal
		default:
			errDef = cerrdefs.ErrUnknown
		}
	}
	return &httpError{
		err:    err,
		errDef: errDef,
	}
}
