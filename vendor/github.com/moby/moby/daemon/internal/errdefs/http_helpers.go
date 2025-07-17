package errdefs

import (
	"net/http"
)

// FromStatusCode creates an errdef error, based on the provided HTTP status-code
//
// Deprecated: Use [cerrdefs.ToNative] instead
func FromStatusCode(err error, statusCode int) error {
	if err == nil {
		return nil
	}
	switch statusCode {
	case http.StatusNotFound:
		return NotFound(err)
	case http.StatusBadRequest:
		return InvalidParameter(err)
	case http.StatusConflict:
		return Conflict(err)
	case http.StatusUnauthorized:
		return Unauthorized(err)
	case http.StatusServiceUnavailable:
		return Unavailable(err)
	case http.StatusForbidden:
		return Forbidden(err)
	case http.StatusNotModified:
		return NotModified(err)
	case http.StatusNotImplemented:
		return NotImplemented(err)
	case http.StatusInternalServerError:
		if IsCancelled(err) || IsSystem(err) || IsUnknown(err) || IsDataLoss(err) || IsDeadline(err) {
			return err
		}
		return System(err)
	default:
		switch {
		case statusCode >= http.StatusOK && statusCode < http.StatusBadRequest:
			// it's a client error
			return err
		case statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError:
			return InvalidParameter(err)
		case statusCode >= http.StatusInternalServerError && statusCode < 600:
			return System(err)
		default:
			return Unknown(err)
		}
	}
}
