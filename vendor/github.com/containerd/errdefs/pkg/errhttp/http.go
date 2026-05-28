/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package errhttp provides utility functions for translating errors to
// and from a HTTP context.
//
// The functions ToHTTP and ToNative can be used to map server-side and
// client-side errors to the correct types.
package errhttp

import (
	"errors"
	"net/http"

	"github.com/containerd/errdefs"
	"github.com/containerd/errdefs/pkg/internal/cause"
)

// ToHTTP returns the best status code for the given error
func ToHTTP(err error) int {
	switch {
	case errdefs.IsNotFound(err):
		return http.StatusNotFound
	case errdefs.IsInvalidArgument(err):
		return http.StatusBadRequest
	case errdefs.IsConflict(err):
		return http.StatusConflict
	case errdefs.IsNotModified(err):
		return http.StatusNotModified
	case errdefs.IsFailedPrecondition(err):
		return http.StatusPreconditionFailed
	case errdefs.IsUnauthorized(err):
		return http.StatusUnauthorized
	case errdefs.IsPermissionDenied(err):
		return http.StatusForbidden
	case errdefs.IsResourceExhausted(err):
		return http.StatusTooManyRequests
	case errdefs.IsInternal(err):
		return http.StatusInternalServerError
	case errdefs.IsNotImplemented(err):
		return http.StatusNotImplemented
	case errdefs.IsUnavailable(err):
		return http.StatusServiceUnavailable
	case errdefs.IsUnknown(err):
		var unexpected cause.ErrUnexpectedStatus
		if errors.As(err, &unexpected) && unexpected.Status >= 200 && unexpected.Status < 600 {
			return unexpected.Status
		}
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// ToNative returns the error best matching the HTTP status code
func ToNative(statusCode int) error {
	switch statusCode {
	case http.StatusNotFound:
		return errdefs.ErrNotFound
	case http.StatusBadRequest:
		return errdefs.ErrInvalidArgument
	case http.StatusConflict:
		return errdefs.ErrConflict
	case http.StatusPreconditionFailed:
		return errdefs.ErrFailedPrecondition
	case http.StatusUnauthorized:
		return errdefs.ErrUnauthenticated
	case http.StatusForbidden:
		return errdefs.ErrPermissionDenied
	case http.StatusNotModified:
		return errdefs.ErrNotModified
	case http.StatusTooManyRequests:
		return errdefs.ErrResourceExhausted
	case http.StatusInternalServerError:
		return errdefs.ErrInternal
	case http.StatusNotImplemented:
		return errdefs.ErrNotImplemented
	case http.StatusServiceUnavailable:
		return errdefs.ErrUnavailable
	default:
		return cause.ErrUnexpectedStatus{Status: statusCode}
	}
}
