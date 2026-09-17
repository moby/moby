package httpstatus

import (
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
)

// TestAllErrdefsTypesAreMapped ensures that all containerd errdefs error types
// are explicitly mapped to HTTP status codes in FromError.
// This test serves as a safeguard against adding new errdefs types without
// corresponding HTTP status mappings.
func TestAllErrdefsTypesAreMapped(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
	}{
		{
			name:           "NotFound",
			err:            cerrdefs.ErrNotFound,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "InvalidArgument",
			err:            cerrdefs.ErrInvalidArgument,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Conflict",
			err:            cerrdefs.ErrConflict,
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "AlreadyExists",
			err:            cerrdefs.ErrAlreadyExists,
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "FailedPrecondition",
			err:            cerrdefs.ErrFailedPrecondition,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "OutOfRange",
			err:            cerrdefs.ErrOutOfRange,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Aborted",
			err:            cerrdefs.ErrAborted,
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "ResourceExhausted",
			err:            cerrdefs.ErrResourceExhausted,
			expectedStatus: http.StatusTooManyRequests,
		},
		{
			name:           "Unauthorized",
			err:            cerrdefs.ErrUnauthenticated,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Unavailable",
			err:            cerrdefs.ErrUnavailable,
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "PermissionDenied",
			err:            cerrdefs.ErrPermissionDenied,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "NotModified",
			err:            cerrdefs.ErrNotModified,
			expectedStatus: http.StatusNotModified,
		},
		{
			name:           "NotImplemented",
			err:            cerrdefs.ErrNotImplemented,
			expectedStatus: http.StatusNotImplemented,
		},
		{
			name:           "Internal",
			err:            cerrdefs.ErrInternal,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "DataLoss",
			err:            cerrdefs.ErrDataLoss,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "Unknown",
			err:            cerrdefs.ErrUnknown,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := FromError(tt.err)
			if status != tt.expectedStatus {
				t.Errorf("FromError(%v) = %d, want %d", tt.name, status, tt.expectedStatus)
			}
		})
	}
}
