package images

import (
	"errors"
	"testing"

	"github.com/moby/moby/v2/errdefs"
	"gotest.tools/v3/assert"
)

func TestCategorizeImageDeleteError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "not found error",
			err:      errdefs.NotFound(errors.New("not found")),
			expected: "not_found",
		},
		{
			name:     "conflict error",
			err:      errdefs.Conflict(errors.New("conflict")),
			expected: "conflict",
		},
		{
			name:     "unauthorized error",
			err:      errdefs.Unauthorized(errors.New("unauthorized")),
			expected: "permission_denied",
		},
		{
			name:     "forbidden error",
			err:      errdefs.Forbidden(errors.New("forbidden")),
			expected: "permission_denied",
		},
		{
			name:     "invalid parameter error",
			err:      errdefs.InvalidParameter(errors.New("invalid")),
			expected: "invalid_argument",
		},
		{
			name:     "wrapped not found error",
			err:      errdefs.NotFound(errors.New("image does not exist")),
			expected: "not_found",
		},
		{
			name:     "wrapped conflict error",
			err:      errdefs.Conflict(errors.New("container is using image")),
			expected: "conflict",
		},
		{
			name:     "unknown error",
			err:      errors.New("some random error"),
			expected: "unknown",
		},
		{
			name:     "nil error",
			err:      nil,
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := categorizeImageDeleteError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
