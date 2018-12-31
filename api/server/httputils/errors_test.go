package httputils

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/docker/docker/errdefs"
	"gotest.tools/assert"
)

func TestFromStatusCode(t *testing.T) {
	testErr := fmt.Errorf("some error occurred")

	testCases := []struct {
		err    error
		status int
		check  func(error) bool
	}{
		{
			err:    testErr,
			status: http.StatusNotFound,
			check:  errdefs.IsNotFound,
		},
		{
			err:    testErr,
			status: http.StatusBadRequest,
			check:  errdefs.IsInvalidParameter,
		},
		{
			err:    testErr,
			status: http.StatusConflict,
			check:  errdefs.IsConflict,
		},
		{
			err:    testErr,
			status: http.StatusUnauthorized,
			check:  errdefs.IsUnauthorized,
		},
		{
			err:    testErr,
			status: http.StatusServiceUnavailable,
			check:  errdefs.IsUnavailable,
		},
		{
			err:    testErr,
			status: http.StatusForbidden,
			check:  errdefs.IsForbidden,
		},
		{
			err:    testErr,
			status: http.StatusNotModified,
			check:  errdefs.IsNotModified,
		},
		{
			err:    testErr,
			status: http.StatusNotImplemented,
			check:  errdefs.IsNotImplemented,
		},
		{
			err:    testErr,
			status: http.StatusInternalServerError,
			check:  errdefs.IsSystem,
		},
		{
			err:    errdefs.Unknown(testErr),
			status: http.StatusInternalServerError,
			check:  errdefs.IsUnknown,
		},
		{
			err:    errdefs.DataLoss(testErr),
			status: http.StatusInternalServerError,
			check:  errdefs.IsDataLoss,
		},
		{
			err:    errdefs.Deadline(testErr),
			status: http.StatusInternalServerError,
			check:  errdefs.IsDeadline,
		},
		{
			err:    errdefs.Cancelled(testErr),
			status: http.StatusInternalServerError,
			check:  errdefs.IsCancelled,
		},
	}

	for _, tc := range testCases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			err := FromStatusCode(tc.err, tc.status)
			assert.Check(t, tc.check(err), "unexpected error-type %T", err)
		})
	}
}
